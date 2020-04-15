/*
Copyright 2019 The Kubernetes Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package disks

import (
	"context"

	"github.com/Azure/azure-sdk-for-go/services/compute/mgmt/2019-12-01/compute"
	"github.com/Azure/go-autorest/autorest"
	azure "sigs.k8s.io/cluster-api-provider-azure/cloud"
)

// Client wraps go-sdk
type Client interface {
	Delete(context.Context, string, string) error
}

// AzureClient contains the Azure go-sdk Client
type AzureClient struct {
	disks compute.DisksClient
}

var _ Client = &AzureClient{}

// NewClient creates a new VM client from subscription ID.
func NewClient(subscriptionID string, authorizer autorest.Authorizer) *AzureClient {
	c := newDisksClient(subscriptionID, authorizer)
	return &AzureClient{c}
}

// newDisksClient creates a new disks client from subscription ID.
func newDisksClient(subscriptionID string, authorizer autorest.Authorizer) compute.DisksClient {
	disksClient := compute.NewDisksClientWithBaseURI(azure.DefaultBaseURI, subscriptionID)
	disksClient.Authorizer = authorizer
	disksClient.AddToUserAgent(azure.UserAgent)
	return disksClient
}

func (ac *AzureClient) Delete(ctx context.Context, resourceGroupName, name string) error {
	future, err := ac.disks.Delete(ctx, resourceGroupName, name)
	if err != nil {
		return err
	}
	err = future.WaitForCompletionRef(ctx, ac.disks.Client)
	if err != nil {
		return err
	}
	_, err = future.Result(ac.disks)
	return err
}
