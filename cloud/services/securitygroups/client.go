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

package securitygroups

import (
	"context"

	"github.com/Azure/azure-sdk-for-go/services/network/mgmt/2019-06-01/network"
	"github.com/Azure/go-autorest/autorest"
	azure "sigs.k8s.io/cluster-api-provider-azure/cloud"
)

// Client wraps go-sdk
type Client interface {
	Get(context.Context, string, string) (network.SecurityGroup, error)
	CreateOrUpdate(context.Context, string, string, network.SecurityGroup) error
	Delete(context.Context, string, string) error
}

// AzureClient contains the Azure go-sdk Client
type AzureClient struct {
	securitygroups network.SecurityGroupsClient
}

var _ Client = &AzureClient{}

// NewClient creates a new VM client from subscription ID.
func NewClient(subscriptionID string, authorizer autorest.Authorizer) *AzureClient {
	c := newSecurityGroupsClient(subscriptionID, authorizer)
	return &AzureClient{c}
}

// newSecurityGroupsClient creates a new security groups client from subscription ID.
func newSecurityGroupsClient(subscriptionID string, authorizer autorest.Authorizer) network.SecurityGroupsClient {
	securityGroupsClient := network.NewSecurityGroupsClientWithBaseURI(azure.DefaultBaseURI, subscriptionID)
	securityGroupsClient.Authorizer = authorizer
	securityGroupsClient.AddToUserAgent(azure.UserAgent)
	return securityGroupsClient
}

// Get gets the specified network security group.
func (ac *AzureClient) Get(ctx context.Context, resourceGroupName, sgName string) (network.SecurityGroup, error) {
	return ac.securitygroups.Get(ctx, resourceGroupName, sgName, "")
}

// CreateOrUpdate creates or updates a network security group in the specified resource group.
func (ac *AzureClient) CreateOrUpdate(ctx context.Context, resourceGroupName string, sgName string, sg network.SecurityGroup) error {
	future, err := ac.securitygroups.CreateOrUpdate(ctx, resourceGroupName, sgName, sg)
	if err != nil {
		return err
	}
	err = future.WaitForCompletionRef(ctx, ac.securitygroups.Client)
	if err != nil {
		return err
	}
	_, err = future.Result(ac.securitygroups)
	return err
}

// Delete deletes the specified network security group.
func (ac *AzureClient) Delete(ctx context.Context, resourceGroupName, sgName string) error {
	future, err := ac.securitygroups.Delete(ctx, resourceGroupName, sgName)
	if err != nil {
		return err
	}
	err = future.WaitForCompletionRef(ctx, ac.securitygroups.Client)
	if err != nil {
		return err
	}
	_, err = future.Result(ac.securitygroups)
	return err
}
