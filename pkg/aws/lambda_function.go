package aws

import (
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/lambda"
	log "github.com/sirupsen/logrus"

	"github.com/Qovery/pleco/pkg/common"
)

type lambdaFunction struct {
	common.CloudProviderResource
}

func LambdaSession(sess session.Session, region string) *lambda.Lambda {
	return lambda.New(&sess, &aws.Config{Region: aws.String(region)})
}

func tagFunctions(svc lambda.Lambda, functions *lambda.ListFunctionsOutput, tagName string) []lambdaFunction {
	var taggedFunctions []lambdaFunction

	for _, function := range functions.Functions {
		input := &lambda.GetFunctionInput{
			FunctionName: function.FunctionName,
		}
		getFunctionResult, err := svc.GetFunction(input)
		if err != nil {
			continue
		}

		essentialTags := common.GetEssentialTags(getFunctionResult.Tags, tagName)

		taggedFunctions = append(taggedFunctions, lambdaFunction{
			CloudProviderResource: common.CloudProviderResource{
				Identifier:   *function.FunctionName,
				Description:  "Lambda Function: " + *function.FunctionName,
				CreationDate: essentialTags.CreationDate,
				TTL:          essentialTags.TTL,
				Tag:          essentialTags.Tag,
				IsProtected:  essentialTags.IsProtected,
			},
		})

	}

	return taggedFunctions
}

func listTaggedFunctions(svc lambda.Lambda, tagName string) ([]lambdaFunction, error) {

	result, err := svc.ListFunctions(nil)

	if err != nil {
		return nil, err
	}

	// No functions, return nil
	if len(result.Functions) == 0 {
		return nil, nil
	}

	taggedFunctions := tagFunctions(svc, result, tagName)

	for result.NextMarker != nil {
		result, err = svc.ListFunctions(&lambda.ListFunctionsInput{
			Marker: result.NextMarker,
		})

		if err != nil {
			return nil, err
		}

		taggedFunctions = append(taggedFunctions, tagFunctions(svc, result, tagName)...)
	}

	return taggedFunctions, nil
}

func deleteLambdaFunction(svc lambda.Lambda, function lambdaFunction) error {
	_, err := svc.DeleteFunction(&lambda.DeleteFunctionInput{
		FunctionName: &function.Identifier,
	},
	)
	if err != nil {
		return err
	}

	return nil
}

func getExpiredLambdaFunctions(ECsession *lambda.Lambda, options *AwsOptions) ([]lambdaFunction, string) {
	functions, err := listTaggedFunctions(*ECsession, options.TagName)
	region := *ECsession.Config.Region
	if err != nil {
		log.Errorf("Can't list Lambda Functions in region %s: %s", region, err.Error())
	}

	var expiredFunctions []lambdaFunction
	for _, function := range functions {
		log.Infof("Checking if %s function is expired", function.Identifier)
		if function.IsResourceExpired(options.TagValue, options.DisableTTLCheck) {
			expiredFunctions = append(expiredFunctions, function)
		}
	}

	return expiredFunctions, region
}

func DeleteExpiredLambdaFunctions(sessions AWSSessions, options AwsOptions) {
	expiredFunctions, region := getExpiredLambdaFunctions(sessions.LambdaFunction, &options)

	count, start := common.ElemToDeleteFormattedInfos("expired Lambda Function", len(expiredFunctions), region)

	log.Info(count)

	if options.DryRun || len(expiredFunctions) == 0 {
		return
	}

	log.Info(start)

	for _, function := range expiredFunctions {
		deletionErr := deleteLambdaFunction(*sessions.LambdaFunction, function)
		if deletionErr != nil {
			log.Errorf("Deletion Lambda function error %s/%s: %s", function.Identifier, region, deletionErr.Error())
		} else {
			log.Debugf("Lambda function %s in %s deleted.", function.Identifier, region)
		}
	}
}
