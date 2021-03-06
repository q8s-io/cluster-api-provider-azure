/*
Copyright 2020 The Kubernetes Authors.

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

package routetables

import (
	"context"
	"net/http"
	"testing"

	. "github.com/onsi/gomega"
	"sigs.k8s.io/cluster-api-provider-azure/cloud/services/routetables/mock_routetables"

	"github.com/Azure/go-autorest/autorest"
	"github.com/golang/mock/gomock"

	network "github.com/Azure/azure-sdk-for-go/services/network/mgmt/2019-06-01/network"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/scheme"
	infrav1 "sigs.k8s.io/cluster-api-provider-azure/api/v1alpha3"
	"sigs.k8s.io/cluster-api-provider-azure/cloud/scope"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1alpha3"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

const expectedInvalidSpec = "invalid Route Table Specification"

func init() {
	clusterv1.AddToScheme(scheme.Scheme)
}

func TestInvalidRouteTableSpec(t *testing.T) {
	g := NewWithT(t)

	mockCtrl := gomock.NewController(t)
	routetableMock := mock_routetables.NewMockClient(mockCtrl)

	cluster := &clusterv1.Cluster{
		ObjectMeta: metav1.ObjectMeta{Name: "test-cluster"},
	}

	client := fake.NewFakeClient(cluster)

	clusterScope, err := scope.NewClusterScope(scope.ClusterScopeParams{
		AzureClients: scope.AzureClients{
			SubscriptionID: "123",
			Authorizer:     autorest.NullAuthorizer{},
		},
		Client:  client,
		Cluster: cluster,
		AzureCluster: &infrav1.AzureCluster{
			Spec: infrav1.AzureClusterSpec{
				Location: "test-location",
				ResourceGroup: "my-rg",
				NetworkSpec: infrav1.NetworkSpec{
					Vnet: infrav1.VnetSpec{Name: "my-vnet", ResourceGroup: "my-rg"},
				},
			},
		},
	})
	g.Expect(err).NotTo(HaveOccurred())

	s := &Service{
		Scope:  clusterScope,
		Client: routetableMock,
	}

	// Wrong Spec
	wrongSpec := &network.PublicIPAddress{}

	_, err = s.Get(context.TODO(), &wrongSpec)
	g.Expect(err).To(HaveOccurred())
	g.Expect(err).To(MatchError(expectedInvalidSpec))

	err = s.Reconcile(context.TODO(), &wrongSpec)
	g.Expect(err).To(HaveOccurred())
	g.Expect(err).To(MatchError(expectedInvalidSpec))

	err = s.Delete(context.TODO(), &wrongSpec)
	g.Expect(err).To(HaveOccurred())
	g.Expect(err).To(MatchError(expectedInvalidSpec))
}

func TestGetRouteTable(t *testing.T) {
	g := NewWithT(t)

	testcases := []struct {
		name           string
		routetableSpec Spec
		expectedError  string
		expect         func(m *mock_routetables.MockClientMockRecorder)
	}{
		{
			name: "get existing route table",
			routetableSpec: Spec{
				Name: "my-routetable",
			},
			expectedError: "",
			expect: func(m *mock_routetables.MockClientMockRecorder) {
				m.Get(context.TODO(), "my-rg", "my-routetable").Return(network.RouteTable{}, nil)
			},
		},
		{
			name: "route table not found",
			routetableSpec: Spec{
				Name: "my-routetable",
			},
			expectedError: "route table my-routetable not found: #: Not found: StatusCode=404",
			expect: func(m *mock_routetables.MockClientMockRecorder) {
				m.Get(context.TODO(), "my-rg", "my-routetable").Return(network.RouteTable{}, autorest.NewErrorWithResponse("", "", &http.Response{StatusCode: 404}, "Not found"))
			},
		},
		{
			name: "route table retrieval fails",
			routetableSpec: Spec{
				Name: "my-routetable",
			},
			expectedError: "#: Internal Server Error: StatusCode=500",
			expect: func(m *mock_routetables.MockClientMockRecorder) {
				m.Get(context.TODO(), "my-rg", "my-routetable").Return(network.RouteTable{}, autorest.NewErrorWithResponse("", "", &http.Response{StatusCode: 500}, "Internal Server Error"))
			},
		},
	}

	for _, tc := range testcases {
		t.Run(tc.name, func(t *testing.T) {
			mockCtrl := gomock.NewController(t)
			routetableMock := mock_routetables.NewMockClient(mockCtrl)

			cluster := &clusterv1.Cluster{
				ObjectMeta: metav1.ObjectMeta{Name: "test-cluster"},
			}

			client := fake.NewFakeClient(cluster)

			tc.expect(routetableMock.EXPECT())

			clusterScope, err := scope.NewClusterScope(scope.ClusterScopeParams{
				AzureClients: scope.AzureClients{
					SubscriptionID: "123",
					Authorizer:     autorest.NullAuthorizer{},
				},
				Client:  client,
				Cluster: cluster,
				AzureCluster: &infrav1.AzureCluster{
					Spec: infrav1.AzureClusterSpec{
						Location: "test-location",
						ResourceGroup: "my-rg",
						NetworkSpec: infrav1.NetworkSpec{
							Vnet: infrav1.VnetSpec{Name: "my-vnet", ResourceGroup: "my-rg"},
						},
					},
				},
			})
			g.Expect(err).NotTo(HaveOccurred())

			s := &Service{
				Scope:  clusterScope,
				Client: routetableMock,
			}

			_, err = s.Get(context.TODO(), &tc.routetableSpec)
			if tc.expectedError != "" {
				g.Expect(err).To(HaveOccurred())
				g.Expect(err).To(MatchError(tc.expectedError))
			} else {
				g.Expect(err).NotTo(HaveOccurred())
			}
		})
	}
}

func TestReconcileRouteTables(t *testing.T) {
	g := NewWithT(t)

	testcases := []struct {
		name           string
		routetableSpec Spec
		tags           infrav1.Tags
		expectedError  string
		expect         func(m *mock_routetables.MockClientMockRecorder)
	}{
		{
			name: "route tables in custom vnet mode",
			routetableSpec: Spec{
				Name: "my-routetable",
			},
			tags: infrav1.Tags{
				"Name": "my-vnet",
				"sigs.k8s.io_cluster-api-provider-azure_cluster_test-cluster": "shared",
				"sigs.k8s.io_cluster-api-provider-azure_role":                 "common",
			},
			expectedError: "",
			expect: func(m *mock_routetables.MockClientMockRecorder) {
				m.CreateOrUpdate(context.TODO(), "my-rg", "my-routetable", gomock.AssignableToTypeOf(network.RouteTable{}))
			},
		},
		{
			name: "route table create successfully",
			routetableSpec: Spec{
				Name: "my-routetable",
			},
			tags: infrav1.Tags{
				"Name": "my-vnet",
				"sigs.k8s.io_cluster-api-provider-azure_cluster_test-cluster": "owned",
				"sigs.k8s.io_cluster-api-provider-azure_role":                 "common",
			},
			expectedError: "",
			expect: func(m *mock_routetables.MockClientMockRecorder) {
				m.CreateOrUpdate(context.TODO(), "my-rg", "my-routetable", gomock.AssignableToTypeOf(network.RouteTable{}))
			},
		},
		{
			name: "fail to create a route table",
			routetableSpec: Spec{
				Name: "my-routetable",
			},
			tags: infrav1.Tags{
				"Name": "my-vnet",
				"sigs.k8s.io_cluster-api-provider-azure_cluster_test-cluster": "owned",
				"sigs.k8s.io_cluster-api-provider-azure_role":                 "common",
			},
			expectedError: "failed to create route table my-routetable in resource group my-rg: #: Internal Server Error: StatusCode=500",
			expect: func(m *mock_routetables.MockClientMockRecorder) {
				m.CreateOrUpdate(context.TODO(), "my-rg", "my-routetable", gomock.AssignableToTypeOf(network.RouteTable{})).Return(autorest.NewErrorWithResponse("", "", &http.Response{StatusCode: 500}, "Internal Server Error"))
			},
		},
	}

	for _, tc := range testcases {
		t.Run(tc.name, func(t *testing.T) {
			mockCtrl := gomock.NewController(t)
			routetableMock := mock_routetables.NewMockClient(mockCtrl)

			cluster := &clusterv1.Cluster{
				ObjectMeta: metav1.ObjectMeta{Name: "test-cluster"},
			}

			client := fake.NewFakeClient(cluster)

			tc.expect(routetableMock.EXPECT())

			clusterScope, err := scope.NewClusterScope(scope.ClusterScopeParams{
				AzureClients: scope.AzureClients{
					SubscriptionID: "123",
					Authorizer:     autorest.NullAuthorizer{},
				},
				Client:  client,
				Cluster: cluster,
				AzureCluster: &infrav1.AzureCluster{
					Spec: infrav1.AzureClusterSpec{
						Location: "test-location",
						ResourceGroup: "my-rg",
						NetworkSpec: infrav1.NetworkSpec{
							Vnet: infrav1.VnetSpec{
								ID:            "my-vnet-id",
								Name:          "my-vnet",
								ResourceGroup: "my-rg",
								Tags:          tc.tags,
							},
						},
					},
				},
			})
			g.Expect(err).NotTo(HaveOccurred())

			s := &Service{
				Scope:  clusterScope,
				Client: routetableMock,
			}

			err = s.Reconcile(context.TODO(), &tc.routetableSpec)
			if tc.expectedError != "" {
				g.Expect(err).To(HaveOccurred())
				g.Expect(err).To(MatchError(tc.expectedError))
			} else {
				g.Expect(err).NotTo(HaveOccurred())
			}
		})
	}
}

func TestDeleteRouteTable(t *testing.T) {
	g := NewWithT(t)

	testcases := []struct {
		name           string
		routetableSpec Spec
		tags           infrav1.Tags
		expectedError  string
		expect         func(m *mock_routetables.MockClientMockRecorder)
	}{
		{
			name: "route tables in custom vnet mode",
			routetableSpec: Spec{
				Name: "my-routetable",
			},
			tags: infrav1.Tags{
				"Name": "my-vnet",
				"sigs.k8s.io_cluster-api-provider-azure_cluster_test-cluster": "shared",
				"sigs.k8s.io_cluster-api-provider-azure_role":                 "common",
			},
			expectedError: "",
			expect: func(m *mock_routetables.MockClientMockRecorder) {
				m.Delete(context.TODO(), "my-rg", "my-routetable")
			},
		},
		{
			name: "route table deleted successfully",
			routetableSpec: Spec{
				Name: "my-routetable",
			},
			tags: infrav1.Tags{
				"Name": "my-vnet",
				"sigs.k8s.io_cluster-api-provider-azure_cluster_test-cluster": "owned",
				"sigs.k8s.io_cluster-api-provider-azure_role":                 "common",
			},
			expectedError: "",
			expect: func(m *mock_routetables.MockClientMockRecorder) {
				m.Delete(context.TODO(), "my-rg", "my-routetable")
			},
		},
		{
			name: "route table already deleted",
			routetableSpec: Spec{
				Name: "my-routetable",
			},
			tags: infrav1.Tags{
				"Name": "my-vnet",
				"sigs.k8s.io_cluster-api-provider-azure_cluster_test-cluster": "owned",
				"sigs.k8s.io_cluster-api-provider-azure_role":                 "common",
			},
			expectedError: "",
			expect: func(m *mock_routetables.MockClientMockRecorder) {
				m.Delete(context.TODO(), "my-rg", "my-routetable").Return(autorest.NewErrorWithResponse("", "", &http.Response{StatusCode: 404}, "Not Found"))
			},
		},
		{
			name: "route table deletion fails",
			routetableSpec: Spec{
				Name: "my-routetable",
			},
			tags: infrav1.Tags{
				"Name": "my-vnet",
				"sigs.k8s.io_cluster-api-provider-azure_cluster_test-cluster": "owned",
				"sigs.k8s.io_cluster-api-provider-azure_role":                 "common",
			},
			expectedError: "failed to delete route table my-routetable in resource group my-rg: #: Internal Server Error: StatusCode=500",
			expect: func(m *mock_routetables.MockClientMockRecorder) {
				m.Delete(context.TODO(), "my-rg", "my-routetable").Return(autorest.NewErrorWithResponse("", "", &http.Response{StatusCode: 500}, "Internal Server Error"))
			},
		},
	}

	for _, tc := range testcases {
		t.Run(tc.name, func(t *testing.T) {
			mockCtrl := gomock.NewController(t)
			routetableMock := mock_routetables.NewMockClient(mockCtrl)

			cluster := &clusterv1.Cluster{
				ObjectMeta: metav1.ObjectMeta{Name: "test-cluster"},
			}

			client := fake.NewFakeClient(cluster)

			tc.expect(routetableMock.EXPECT())

			clusterScope, err := scope.NewClusterScope(scope.ClusterScopeParams{
				AzureClients: scope.AzureClients{
					SubscriptionID: "123",
					Authorizer:     autorest.NullAuthorizer{},
				},
				Client:  client,
				Cluster: cluster,
				AzureCluster: &infrav1.AzureCluster{
					Spec: infrav1.AzureClusterSpec{
						Location: "test-location",
						ResourceGroup: "my-rg",
						NetworkSpec: infrav1.NetworkSpec{
							Vnet: infrav1.VnetSpec{
								ID:            "my-vnet-id",
								Name:          "my-vnet",
								ResourceGroup: "my-rg",
								Tags:          tc.tags,
							},
						},
					},
				},
			})
			g.Expect(err).NotTo(HaveOccurred())

			s := &Service{
				Scope:  clusterScope,
				Client: routetableMock,
			}

			err = s.Delete(context.TODO(), &tc.routetableSpec)
			if tc.expectedError != "" {
				g.Expect(err).To(HaveOccurred())
				g.Expect(err).To(MatchError(tc.expectedError))
			} else {
				g.Expect(err).NotTo(HaveOccurred())
			}
		})
	}
}
