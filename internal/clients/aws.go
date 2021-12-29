package aws

import (
	"context"
	"fmt"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/credentials/stscreds"
	"github.com/aws/aws-sdk-go-v2/service/sts"
	"github.com/aws/smithy-go"
	xpv1 "github.com/crossplane/crossplane-runtime/apis/common/v1"
	"github.com/crossplane/crossplane-runtime/pkg/resource"
	"github.com/pkg/errors"
	"gopkg.in/ini.v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/pointer"
	"provider-aws-controlapi/apis/v1beta1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"strconv"
	"strings"
)

// DefaultSection for INI files.
const DefaultSection = ini.DefaultSection

// GlobalRegion is the region name used for AWS services that do not have a notion
// of region.
const GlobalRegion = "aws-global"

// Endpoint URL configuration types.
const (
	URLConfigTypeStatic  = "Static"
	URLConfigTypeDynamic = "Dynamic"
)


// GetConfig constructs an *aws.Config that can be used to authenticate to AWS
// API by the AWS clients.
func GetConfig(ctx context.Context, c client.Client, mg resource.Managed, region string) (*aws.Config, error) {
	switch {
	case mg.GetProviderConfigReference() != nil:
		return UseProviderConfig(ctx, c, mg, region)
	default:
		return nil, errors.New("neither providerConfigRef nor providerRef is given")
	}
}

// UseProviderConfig to produce a config that can be used to authenticate to AWS.
func UseProviderConfig(ctx context.Context, c client.Client, mg resource.Managed, region string) (*aws.Config, error) { // nolint:gocyclo
	pc := &v1beta1.ProviderConfig{}
	if err := c.Get(ctx, types.NamespacedName{Name: mg.GetProviderConfigReference().Name}, pc); err != nil {
		return nil, errors.Wrap(err, "cannot get referenced Provider")
	}

	t := resource.NewProviderConfigUsageTracker(c, &v1beta1.ProviderConfigUsage{})
	if err := t.Track(ctx, mg); err != nil {
		return nil, errors.Wrap(err, "cannot track ProviderConfig usage")
	}

	switch s := pc.Spec.Credentials.Source; s { //nolint:exhaustive
	case xpv1.CredentialsSourceInjectedIdentity:
		if pc.Spec.AssumeRoleARN != nil {
			cfg, err := UsePodServiceAccountAssumeRole(ctx, []byte{}, DefaultSection, region, pc)
			if err != nil {
				return nil, err
			}
			return SetResolver(pc, cfg), nil
		}
		cfg, err := UsePodServiceAccount(ctx, []byte{}, DefaultSection, region)
		if err != nil {
			return nil, err
		}
		return SetResolver(pc, cfg), nil
	default:
		data, err := resource.CommonCredentialExtractor(ctx, s, c, pc.Spec.Credentials.CommonCredentialSelectors)
		if err != nil {
			return nil, errors.Wrap(err, "cannot get credentials")
		}
		if pc.Spec.AssumeRoleARN != nil {
			cfg, err := UseProviderSecretAssumeRole(ctx, data, DefaultSection, region, pc)
			if err != nil {
				return nil, err
			}
			return SetResolver(pc, cfg), nil
		}
		cfg, err := UseProviderSecret(ctx, data, DefaultSection, region)
		if err != nil {
			return nil, err
		}
		return SetResolver(pc, cfg), nil
	}
}

// UsePodServiceAccountAssumeRole assumes an IAM role configured via a ServiceAccount
// assume Cross account IAM roles
// https://aws.amazon.com/blogs/containers/cross-account-iam-roles-for-kubernetes-service-accounts/
func UsePodServiceAccountAssumeRole(ctx context.Context, _ []byte, _, region string, pc *v1beta1.ProviderConfig) (*aws.Config, error) {
	cfg, err := config.LoadDefaultConfig(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "failed to load default AWS config")
	}
	stsclient := sts.NewFromConfig(cfg)
	cnf, err := config.LoadDefaultConfig(
		ctx,
		config.WithRegion(region),
		config.WithCredentialsProvider(aws.NewCredentialsCache(
			stscreds.NewAssumeRoleProvider(
				stsclient,
				StringValue(pc.Spec.AssumeRoleARN),
			)),
		),
	)
	if err != nil {
		return nil, errors.Wrap(err, "failed to load assumed role AWS config")
	}
	return &cnf, err
}

// UsePodServiceAccount assumes an IAM role configured via a ServiceAccount.
// https://docs.aws.amazon.com/eks/latest/userguide/iam-roles-for-service-accounts.html
func UsePodServiceAccount(ctx context.Context, _ []byte, _, region string) (*aws.Config, error) {
	cfg, err := config.LoadDefaultConfig(
		ctx,
		config.WithRegion(region),
	)
	if err != nil {
		return nil, errors.Wrap(err, "failed to load default AWS config")
	}
	return &cfg, err
}


// UseProviderSecretAssumeRole - AWS configuration which can be used to issue requests against AWS API
// assume Cross account IAM roles
func UseProviderSecretAssumeRole(ctx context.Context, data []byte, profile, region string, pc *v1beta1.ProviderConfig) (*aws.Config, error) {
	creds, err := CredentialsIDSecret(data, profile)
	if err != nil {
		return nil, errors.Wrap(err, "cannot parse credentials secret")
	}

	config, err := config.LoadDefaultConfig(ctx, config.WithRegion(region), config.WithCredentialsProvider(credentials.StaticCredentialsProvider{
		Value: creds,
	}))

	stsSvc := sts.NewFromConfig(config)
	stsAssume := stscreds.NewAssumeRoleProvider(stsSvc, StringValue(pc.Spec.AssumeRoleARN))
	config.Credentials = aws.NewCredentialsCache(stsAssume)

	return &config, err
}

// UseProviderSecret - AWS configuration which can be used to issue requests against AWS API
func UseProviderSecret(ctx context.Context, data []byte, profile, region string) (*aws.Config, error) {
	creds, err := CredentialsIDSecret(data, profile)
	if err != nil {
		return nil, errors.Wrap(err, "cannot parse credentials secret")
	}

	config, err := config.LoadDefaultConfig(ctx, config.WithRegion(region), config.WithCredentialsProvider(credentials.StaticCredentialsProvider{
		Value: creds,
	}))
	return &config, err
}

// CredentialsIDSecret retrieves AWS_ACCESS_KEY_ID and AWS_SECRET_ACCESS_KEY from the data which contains
// aws credentials under given profile
// Example:
// [default]
// aws_access_key_id = <YOUR_ACCESS_KEY_ID>
// aws_secret_access_key = <YOUR_SECRET_ACCESS_KEY>
func CredentialsIDSecret(data []byte, profile string) (aws.Credentials, error) {
	config, err := ini.InsensitiveLoad(data)
	if err != nil {
		return aws.Credentials{}, errors.Wrap(err, "cannot parse credentials secret")
	}

	iniProfile, err := config.GetSection(profile)
	if err != nil {
		return aws.Credentials{}, errors.Wrap(err, fmt.Sprintf("cannot get %s profile in credentials secret", profile))
	}

	accessKeyID := iniProfile.Key("aws_access_key_id")
	secretAccessKey := iniProfile.Key("aws_secret_access_key")
	sessionToken := iniProfile.Key("aws_session_token")

	// NOTE(muvaf): Key function implementation never returns nil but still its
	// type is pointer so we check to make sure its next versions doesn't break
	// that implicit contract.
	if accessKeyID == nil || secretAccessKey == nil || sessionToken == nil {
		return aws.Credentials{}, errors.New("returned key can be empty but cannot be nil")
	}

	return aws.Credentials{
		AccessKeyID:     accessKeyID.Value(),
		SecretAccessKey: secretAccessKey.Value(),
		SessionToken:    sessionToken.Value(),
	}, nil
}

type awsEndpointResolverAdaptorWithOptions func(service, region string, options interface{}) (aws.Endpoint, error)

func (a awsEndpointResolverAdaptorWithOptions) ResolveEndpoint(service, region string, options ...interface{}) (aws.Endpoint, error) {
	return a(service, region, options)
}

// SetResolver parses annotations from the managed resource
// and returns a configuration accordingly.
func SetResolver(pc *v1beta1.ProviderConfig, cfg *aws.Config) *aws.Config { // nolint:gocyclo
	if pc.Spec.Endpoint == nil {
		return cfg
	}
	cfg.EndpointResolverWithOptions = awsEndpointResolverAdaptorWithOptions(func(service, region string, options interface{}) (aws.Endpoint, error) {
		fullURL := ""
		switch pc.Spec.Endpoint.URL.Type {
		case URLConfigTypeStatic:
			if pc.Spec.Endpoint.URL.Static == nil {
				return aws.Endpoint{}, errors.New("static type is chosen but static field does not have a value")
			}
			fullURL = StringValue(pc.Spec.Endpoint.URL.Static)
		case URLConfigTypeDynamic:
			if pc.Spec.Endpoint.URL.Dynamic == nil {
				return aws.Endpoint{}, errors.New("dynamic type is chosen but dynamic configuration is not given")
			}
			// NOTE(muvaf): IAM does not have any region.
			if service == "IAM" {
				fullURL = fmt.Sprintf("%s://%s.%s", pc.Spec.Endpoint.URL.Dynamic.Protocol, strings.ToLower(service), pc.Spec.Endpoint.URL.Dynamic.Host)
			} else {
				fullURL = fmt.Sprintf("%s://%s.%s.%s", pc.Spec.Endpoint.URL.Dynamic.Protocol, strings.ToLower(service), region, pc.Spec.Endpoint.URL.Dynamic.Host)
			}
		default:
			return aws.Endpoint{}, errors.New("unsupported url config type is chosen")
		}
		e := aws.Endpoint{
			URL:               fullURL,
			HostnameImmutable: BoolValue(pc.Spec.Endpoint.HostnameImmutable),
			PartitionID:       StringValue(pc.Spec.Endpoint.PartitionID),
			SigningName:       StringValue(pc.Spec.Endpoint.SigningName),
			SigningRegion:     StringValue(LateInitializeStringPtr(pc.Spec.Endpoint.SigningRegion, &region)),
			SigningMethod:     StringValue(pc.Spec.Endpoint.SigningMethod),
		}
		// Only IAM does not have a region parameter and "aws-global" is used in
		// SDK setup. However, signing region has to be us-east-1 and it needs
		// to be set.
		if region == "aws-global" {
			switch StringValue(pc.Spec.Endpoint.PartitionID) {
			case "aws-us-gov", "aws-cn":
				e.SigningRegion = StringValue(LateInitializeStringPtr(pc.Spec.Endpoint.SigningRegion, &region))
			default:
				e.SigningRegion = "us-east-1"
			}
		}
		if pc.Spec.Endpoint.Source != nil {
			switch *pc.Spec.Endpoint.Source {
			case "ServiceMetadata":
				e.Source = aws.EndpointSourceServiceMetadata
			case "Custom":
				e.Source = aws.EndpointSourceCustom
			}
		}
		return e, nil
	})
	return cfg
}



// StringValue converts the supplied string pointer to a string, returning the
// empty string if the pointer is nil.
// TODO(muvaf): is this really meaningful? why not implement it?
func StringValue(v *string) string {
	return aws.ToString(v)
}

// BoolValue calls underlying aws ToBool
func BoolValue(v *bool) bool {
	return aws.ToBool(v)
}

// LateInitializeStringPtr returns in if it's non-nil, otherwise returns from
// which is the backup for the cases in is nil.
func LateInitializeStringPtr(in *string, from *string) *string {
	if in != nil {
		return in
	}
	return from
}

// LateInitializeBoolPtr returns in if it's non-nil, otherwise returns from
// which is the backup for the cases in is nil.
func LateInitializeBoolPtr(in *bool, from *bool) *bool {
	if in != nil {
		return in
	}
	return from
}

// LateInitializeInt32Ptr returns in if it's non-nil, otherwise returns from
// which is the backup for the cases in is nil.
func LateInitializeInt32Ptr(in *int32, from *int32) *int32 {
	if in != nil {
		return in
	}
	return from
}

// LateInitializeInt64Ptr returns in if it's non-nil, otherwise returns from
// which is the backup for the cases in is nil.
func LateInitializeInt64Ptr(in *int64, from *int64) *int64 {
	if in != nil {
		return in
	}
	return from
}

// Wrap will remove the request-specific information from the error and only then
// wrap it.
func Wrap(err error, msg string) error {
	// NOTE(muvaf): nil check is done for performance, otherwise errors.As makes
	// a few reflection calls before returning false, letting awsErr be nil.
	if err == nil {
		return nil
	}
	var awsErr smithy.APIError
	if errors.As(err, &awsErr) {
		return errors.Wrap(awsErr, msg)
	}
	return errors.Wrap(err, msg)
}

// StrToBool convert string to boolean value
func StrToBoolPtr(s string) *bool{
	b,e := strconv.ParseBool(s)
	if e != nil{
		return nil
	}
	return pointer.BoolPtr(b)
}

func StrToIntPtr(s string) *int{
	i,e := strconv.Atoi(s)
	if e != nil{
		return nil
	}
	return pointer.IntPtr(i)
}

