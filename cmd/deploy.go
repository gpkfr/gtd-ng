/*
Copyright © 2020 Guillaume Pancak <gpkfr@imelbox.com>

Permission is hereby granted, free of charge, to any person obtaining a copy
of this software and associated documentation files (the "Software"), to deal
in the Software without restriction, including without limitation the rights
to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
copies of the Software, and to permit persons to whom the Software is
furnished to do so, subject to the following conditions:

The above copyright notice and this permission notice shall be included in
all copies or substantial portions of the Software.

THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN
THE SOFTWARE.
*/
package cmd

import (
	"fmt"
	"log"
	"strings"

	"github.com/aws/aws-sdk-go/service/ecs"
	gtdAWS "github.com/gpkfr/gtd-ng/aws"
	"github.com/spf13/cobra"
)

// deployCmd represents the deploy command
var deployCmd = &cobra.Command{
	Use:   "deploy",
	Short: "Deploy service(s)",
	Long: `Deploy Docker image to AWS ECS. For example:

gtd-ng deploy --env sleep360
deploy services described into 'gtd/sleep360.yaml to a specified ecs`,
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("deploy called")
		deploy()
	},
}

func init() {
	rootCmd.AddCommand(deployCmd)

	// Here you will define your flags and configuration settings.

	// Cobra supports Persistent Flags which will work for this command
	// and all subcommands, e.g.:
	// deployCmd.PersistentFlags().String("foo", "", "A help for foo")

	// Cobra supports local flags which will only run when this command
	// is called directly, e.g.:
	// deployCmd.Flags().BoolP("toggle", "t", false, "Help message for toggle")
	deployCmd.Flags().StringVarP(&gtdenv, "env", "e", "sleep360", "Environment to deploy to")
	deployCmd.Flags().StringVarP(&newContainerImage, "container-image", "c", "", "Container Image to deploy")
	deployCmd.Flags().StringVarP(&newContainerTag, "tag", "t", "", "tag of Image to deploy")
	deployCmd.Flags().StringSliceVarP(&selectedService, "service", "s", []string{}, "Selected service")
}

func deploy() {

	var services gtdAWS.Services
	fmt.Printf("Selected service: %v\n", selectedService)

	err := gtdAWS.LoadService(&services, &gtdenv)
	if err != nil {
		log.Fatal(err)
	}

	//Da AWS Stuff
	awsSession, err := gtdAWS.NewAWSSession(&services.ECSRegion, &awsProfile)
	if err != nil {
		log.Fatal(err)
	}

	svc := ecs.New(awsSession)

	if len(selectedService) <= 0 {
		err = gtdAWS.GetServiceTask(services, svc)
		if err != nil {
			log.Fatal(err)
		}
	} else {
		err = gtdAWS.GetServiceTask(services, svc, selectedService...)
		if err != nil {
			log.Fatal(err)
		}
	}

	for i, s := range services.Services {
		if s.TaskARN != "" {
			currentTask, err := gtdAWS.GetCurrentServiceTaskDefinition(svc, s.TaskARN)
			if err != nil {
				log.Println(err)
			}
			services.Services[i].TaskDefinition = currentTask.TaskDefinition
		}
	}
	fmt.Printf("Current Services Status\n")

	// loop under services
	for _, aService := range services.Services {
		if aService.TaskDefinition != nil {
			fmt.Printf("Services: %s - TaskARN: %s\nFamily: %s\n", aService.Name, aService.TaskARN, *aService.TaskDefinition.Family)
			fmt.Printf("Revision: %d\nActual Image: %s\n", *aService.TaskDefinition.Revision, *aService.TaskDefinition.ContainerDefinitions[0].Image)

			if newContainerImage == "" {
				newContainerImage = aService.Registry
			}

			if newContainerTag != "" {
				if strings.Contains(newContainerImage, ":") {
					log.Fatal(fmt.Errorf("Tags already defined in %s\n", newContainerImage))
				}
				if !strings.Contains(newContainerTag, ":") {
					newContainerTag = fmt.Sprintf(":%s", newContainerTag)
				}
			}
			//update tasks
			if *aService.TaskDefinition.ContainerDefinitions[0].Image != fmt.Sprintf("%s%s", newContainerImage, newContainerTag) {
				input := &ecs.RegisterTaskDefinitionInput{
					ContainerDefinitions: aService.TaskDefinition.ContainerDefinitions,
					Family:               aService.TaskDefinition.Family,
					TaskRoleArn:          aService.TaskDefinition.TaskRoleArn,
				}
				fmt.Println(fmt.Sprintf("Desired Image: %s%s", newContainerImage, newContainerTag))

				input.ContainerDefinitions[0].SetImage(fmt.Sprintf("%s%s", newContainerImage, newContainerTag))
				result, err := svc.RegisterTaskDefinition(input)
				if err != nil {
					log.Fatal(fmt.Errorf("error while registering task definifition : %s\n%s", *aService.TaskDefinition.Family, err.Error()))
				}

				fmt.Printf("Registered new Task Definition: %s:%d\n", *result.TaskDefinition.Family, *result.TaskDefinition.Revision)
				NewServiceTaskDefinition := fmt.Sprintf("%s:%d", *result.TaskDefinition.Family, *result.TaskDefinition.Revision)

				//Update Service
				_, err = gtdAWS.UpdateAWSService(svc, &aService.Name, &services.ECSCluster, &NewServiceTaskDefinition)
				if err != nil {
					log.Println(fmt.Errorf("error while updating service: %s\n %s", aService.Name, err.Error()))
				}
			} else {
				log.Printf("Skipping update of %s, identical image detected: %s/%s\n\n", aService.Name, newContainerImage, *aService.TaskDefinition.ContainerDefinitions[0].Image)
			}
		}
	}
}
