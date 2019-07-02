package main

import (
	"flag"
	"fmt"
	"os"
	"strconv"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	k8s "k8s.io/client-go/kubernetes"
	k8srest "k8s.io/client-go/rest"
	k8sclientcmd "k8s.io/client-go/tools/clientcmd"

	"github.com/ggmaresca/azp-agent-autoscaler/pkg/azuredevops"
)

func main() {
	// Parse arguments
	minStr := flag.String("min", "1", "Minimum number of free agents to keep alive. Minimum of 1.")
	maxStr := flag.String("max", "100", "Maximum number of agents allowed.")
	rateStr := flag.String("rate", "10s", "Duration to check the number of agents.")
	resourceType := flag.String("type", "StatefulSet", "Resource type of the agent. Only StatefulSet is supported.")
	resourceName := flag.String("name", "", "The name of the StatefulSet.")
	resourceNamespace := flag.String("namespace", "", "The namespace of the StatefulSet.")
	azpToken := flag.String("token", "", "The Azure Devops token.")
	azpURL := flag.String("url", "", "The Azure Devops URL. https://dev.azure.com/AccountName")

	flag.Parse()

	min, err := strconv.Atoi(*minStr)
	if err != nil {
		panic(fmt.Sprintf("Error converting min argument to int: %s", err.Error()))
	} else if min < 1 {
		panic("Error - min argument cannot be less than 1.")
	}

	max, err := strconv.Atoi(*maxStr)
	if err != nil {
		panic(fmt.Sprintf("Error converting max argument to int: %s", err.Error()))
	} else if max <= min {
		panic("Error - max pods argument must be greater than the minimum.")
	}

	rate, err := time.ParseDuration(*rateStr)
	if err != nil {
		panic(fmt.Sprintf("Error parsing rate: %s", err.Error()))
	} else if rate.Seconds() <= 1 {
		panic(fmt.Sprintf("Error - rate '%s' is too low.", rate.String()))
	}

	if *resourceType != "StatefulSet" {
		panic(fmt.Sprintf("Error - Unknown resource type %s", *resourceType))
	}

	if *resourceName == "" {
		panic(fmt.Sprintf("Error - %s name is required.", *resourceType))
	}

	if *resourceNamespace == "" {
		panic("Error - namespace is required.")
	}

	if *azpToken == "" {
		panic("Error - the Azure Devops token is required.")
	}

	if *azpURL == "" {
		panic("Error - the Azure Devops URL is required.")
	}

	// Initialize Kubernetes client
	/*k8sConfig, err := clientcmd.BuildConfigFromFlags("", "")
	if err != nil {
		panic("Error initializing Kubernetes config: " + err.Error())
	}*/
	k8sConfig, err := k8srest.InClusterConfig()
	if err != nil {
		kubeconfigEnv := os.Getenv("KUBECONFIG")
		k8sConfig, err = k8sclientcmd.BuildConfigFromFlags("", kubeconfigEnv)
		if err != nil {
			k8sConfig, err = k8sclientcmd.BuildConfigFromFlags("", fmt.Sprintf("%s/.kube/config", homepath()))
			if err != nil {
				panic(fmt.Sprintf("Error initializing Kubernetes config: %s", err.Error()))
			}
		}
	}

	k8sClient, err := k8s.NewForConfig(k8sConfig)
	if err != nil || k8sClient == nil {
		panic(fmt.Sprintf("Error initializing Kubernetes config: %s", err.Error()))
	}

	azdClient := azuredevops.MakeClient(*azpURL, *azpToken)
	pools, err := azdClient.ListPools()
	if err != nil {
		panic(fmt.Sprintf("Error retrieving agent pools: %s", err.Error()))
	} else if len(pools) == 0 {
		panic("Error - did not find any agent pools")
	}

	deployment, err := k8sClient.AppsV1().StatefulSets(*resourceNamespace).Get(*resourceName, metav1.GetOptions{})
	if err != nil {
		panic(fmt.Sprintf("Error retrieving statefulset/%s in namespace %s: %s", *resourceName, *resourceNamespace, err.Error()))
	} else if deployment == nil {
		panic(fmt.Sprintf("Could not find statefulset/%s in namespace %s", *resourceName, *resourceNamespace))
	}

	hpa, err := k8sClient.AppsV1().StatefulSets(*resourceNamespace).GetScale(*resourceName, metav1.GetOptions{})
	if err != nil {
		panic(fmt.Sprintf("Error checking if statefulset/%s has a HorizontalPodAutoscaler: %s", *resourceName, err.Error()))
	} else if hpa != nil {
		panic(fmt.Sprintf("Error: statefulset/%s cannot have a HorizontalPodAutoscaler attached for azp-agent-autoscaler to work.", *resourceName))
	}

	for {
		err = autoscale(k8sClient, azdClient, deployment, min, max)

		time.Sleep(rate)
	}

	println("Exiting azp-agent-autoscaler")
}

func autoscale(k8sClient *k8s.Clientset, azdClient azuredevops.Client, deployment *appsv1.StatefulSet, min int, max int) error {
	return nil
}

func homepath() string {
	home := os.Getenv("HOME")
	if home != "" {
		return home
	}
	return os.Getenv("USERPROFILE") // windows
}
