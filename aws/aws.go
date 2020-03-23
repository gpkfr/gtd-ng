package aws

import (
	"fmt"
	"log"
	"os"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/ecs"
	"gopkg.in/yaml.v3"
)

type Service struct {
	Name           string `yaml:"name"`
	Registry       string `yaml:"registry"`
	Provider       string `yaml:"provider,omitempty"`
	TaskOnly       bool   `yaml:"taskonly,omitempty"`
	TaskARN        string
	Status         string
	RunningCount   int64
	TaskDefinition *ecs.TaskDefinition
}

type Services struct {
	Github     string `yaml:"github,omitempty"`
	ECSCluster string `yaml:"ecs_cluster"`
	ECSRegion  string `yaml:"ecs_region"`
	Services   []Service
}

func exists(path string) (bool, error) {
	_, err := os.Stat(path)
	if err == nil {
		return true, nil
	}
	if os.IsNotExist(err) {
		return false, nil
	}
	return true, err
}

func LoadService(s *Services, e *string) error {
	var configFilePath string

	configFilePath = fmt.Sprintf("gtd/%s.yaml", *e)
	if isExists, _ := exists(configFilePath); !isExists {
		configFilePath = fmt.Sprintf("configs/%s.yaml", *e)
	}

	f, err := os.Open(configFilePath)
	if err != nil {
		log.Fatal(err)
	}
	defer f.Close()
	decoder := yaml.NewDecoder(f)

	//Reject invalid or unknow fields
	decoder.KnownFields(true)

	err = decoder.Decode(s)
	if err != nil {
		log.Fatal(fmt.Errorf("Could not decode config file %s\n", configFilePath))
	}

	return nil
}

func NewAWSSession(region *string, profile *string) (*session.Session, error) {
	var awsConfig *aws.Config

	if *profile != "" {
		awsConfig = &aws.Config{
			Region:      region,
			Credentials: credentials.NewSharedCredentials("", *profile),
		}
		_, err := awsConfig.Credentials.Get()
		if err != nil {
			return nil, err
		}
	} else {
		awsConfig = &aws.Config{
			Region: region,
		}
	}
	s := session.Must(session.NewSession(awsConfig))

	return s, nil
}

func awsMust(result interface{}, err error) (interface{}, error) {

	if err != nil {
		if aerr, ok := err.(awserr.Error); ok {
			switch aerr.Code() {
			case ecs.ErrCodeServerException:
				fmt.Println(ecs.ErrCodeServerException, aerr.Error())
			case ecs.ErrCodeClientException:
				fmt.Println(ecs.ErrCodeClientException, aerr.Error())
			case ecs.ErrCodeInvalidParameterException:
				fmt.Println(ecs.ErrCodeInvalidParameterException, aerr.Error())
			case ecs.ErrCodeClusterNotFoundException:
				fmt.Println(ecs.ErrCodeClusterNotFoundException, aerr.Error())
			case ecs.ErrCodeServiceNotFoundException:
				fmt.Println(ecs.ErrCodeServiceNotFoundException, aerr.Error())
			case ecs.ErrCodeServiceNotActiveException:
				fmt.Println(ecs.ErrCodeServiceNotActiveException, aerr.Error())
			case ecs.ErrCodePlatformUnknownException:
				fmt.Println(ecs.ErrCodePlatformUnknownException, aerr.Error())
			case ecs.ErrCodePlatformTaskDefinitionIncompatibilityException:
				fmt.Println(ecs.ErrCodePlatformTaskDefinitionIncompatibilityException, aerr.Error())
			case ecs.ErrCodeAccessDeniedException:
				fmt.Println(ecs.ErrCodeAccessDeniedException, aerr.Error())
			default:
				fmt.Println(aerr.Error())
			}
		} else {
			fmt.Println(err.Error())
		}
		return nil, err
	}
	return result, nil
}

func UpdateAWSService(svc *ecs.ECS, serviceName, serviceCluster, taskDefinition *string) (*ecs.UpdateServiceOutput, error) {
	input := &ecs.UpdateServiceInput{
		Cluster:            serviceCluster,
		Service:            serviceName,
		ForceNewDeployment: aws.Bool(true),
		TaskDefinition:     taskDefinition,
	}
	result, err := awsMust(svc.UpdateService(input))
	return result.(*ecs.UpdateServiceOutput), err
}

func GetServiceTask(s Services, svc *ecs.ECS, serviceName ...string) error {
	servicesArray := make([]string, 0, 1)

	if nbServices := len(serviceName); nbServices <= 0 {
		//Pickup all Services
		//Then populate servicesArray if not TaskOnly
		for _, s := range s.Services {
			if !s.TaskOnly {
				servicesArray = append(servicesArray, s.Name)
			}
		}
	} else {
		//Pickup serviceName(s) from Services (from config file)
		//then populate servicesArray if not TaskOnly
		for _, name := range serviceName {
			for _, s := range s.Services {
				if !s.TaskOnly && s.Name == name {
					servicesArray = append(servicesArray, s.Name)
				}
			}
		}
	}

	if len(servicesArray) < 1 {
		err := fmt.Errorf("Missing services")
		return err
	}

	//Call Aws
	input := &ecs.DescribeServicesInput{
		Cluster:  aws.String(s.ECSCluster),
		Services: aws.StringSlice(servicesArray),
	}

	result, err := awsMust(svc.DescribeServices(input))
	if err != nil {
		return err
	}

	//then populate Services struct
	if countResult := len(result.(*ecs.DescribeServicesOutput).Services); countResult > 0 {
		found := 0
		for _, awsService := range result.(*ecs.DescribeServicesOutput).Services {
			for i, gtService := range s.Services {
				if gtService.Name == *awsService.ServiceName {
					s.Services[i].TaskARN = *awsService.Deployments[0].TaskDefinition
					s.Services[i].Status = *awsService.Status
					s.Services[i].RunningCount = *awsService.RunningCount
					found++
				}
			}
			if found == countResult {
				break
			}
		}
	}

	return nil
}

func GetCurrentServiceTaskDefinition(svc *ecs.ECS, taskARN string) (*ecs.DescribeTaskDefinitionOutput, error) {
	input := &ecs.DescribeTaskDefinitionInput{
		TaskDefinition: aws.String(taskARN),
	}

	result, err := awsMust(svc.DescribeTaskDefinition(input))
	return result.(*ecs.DescribeTaskDefinitionOutput), err
}
