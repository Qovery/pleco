package aws

import (
	"time"

	"github.com/Qovery/pleco/pkg/common"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/lambda"
	log "github.com/sirupsen/logrus"
)

type lambdaFunction struct {
	FunctionName string
	CreateTime   time.Time
	TTL          int64
	IsProtected  bool
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
			FunctionName: *function.FunctionName,
			CreateTime:   essentialTags.CreationDate,
			TTL:          essentialTags.TTL,
			IsProtected:  essentialTags.IsProtected,
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

	log.Infof("Deleting Lambda Function %s in %s, expired after %d seconds",
				function.FunctionName, *svc.Config.Region, function.TTL)

	_, err := svc.DeleteFunction(&lambda.DeleteFunctionInput{
				FunctionName: &function.FunctionName,
		},
	)
	if err != nil {
		return err
	}

	return nil
}

func getExpiredLambdaFunctions(ECsession *lambda.Lambda, tagName string) ([]lambdaFunction, string) {
	functions, err := listTaggedFunctions(*ECsession, tagName)
	region := *ECsession.Config.Region
	if err != nil {
		log.Errorf("can't list Lambda Functions in region %s: %s", region, err.Error())
	}

	var expiredFunctions []lambdaFunction
	for _, function := range functions {
		log.Infof("Checking if %s function is expired", function.FunctionName)
		if common.CheckIfExpired(function.CreateTime, function.TTL, "lambda: "+function.FunctionName) && !function.IsProtected {
			expiredFunctions = append(expiredFunctions, function)
		}
	}

	return expiredFunctions, region
}

func DeleteExpiredLambdaFunctions(sessions AWSSessions, options AwsOptions) {
	expiredFunctions, region := getExpiredLambdaFunctions(sessions.LambdaFunction, options.TagName)

	count, start := common.ElemToDeleteFormattedInfos("expired Lambda Function", len(expiredFunctions), region)

	log.Debug(count)

	if options.DryRun || len(expiredFunctions) == 0 {
		return
	}

	log.Debug(start)

	for _, function := range expiredFunctions {
		deletionErr := deleteLambdaFunction(*sessions.LambdaFunction, function)
		if deletionErr != nil {
			log.Errorf("Deletion Lambda function error %s/%s: %s", function.FunctionName, region, deletionErr.Error())
		}
	}
}
