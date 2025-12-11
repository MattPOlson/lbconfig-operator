/*
MIT License

Copyright (c) 2022 Carlos Eduardo de Paula

Permission is hereby granted, free of charge, to any person obtaining a copy
of this software and associated documentation files (the "Software"), to deal
in the Software without restriction, including without limitation the rights
to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
copies of the Software, and to permit persons to whom the Software is
furnished to do so, subject to the following conditions:

The above copyright notice and this permission notice shall be included in all
copies or substantial portions of the Software.

THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
SOFTWARE.
*/

package controller_test

import (
	"context"
	"reflect"
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	lbv1 "github.com/carlosedp/lbconfig-operator/api/v1"
	. "github.com/carlosedp/lbconfig-operator/internal/controller/backend/backend_controller"
	_ "github.com/carlosedp/lbconfig-operator/internal/controller/backend/backend_loader"
	d "github.com/carlosedp/lbconfig-operator/internal/controller/backend/dummy"
)

func TestBackendController(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Backend Controller Suite")
}

const (
	dummyHostIP = "1.2.3.4"
	testNodeIP  = "1.1.1.1"
)

var loadBalancer = &lbv1.ExternalLoadBalancer{
	ObjectMeta: metav1.ObjectMeta{
		Name:      "dummy-backend",
		Namespace: "default",
	},
	Spec: lbv1.ExternalLoadBalancerSpec{
		Vip: "10.0.0.1",
		Provider: lbv1.Provider{
			Vendor: "Dummy",
			Host:   dummyHostIP,
			Port:   443,
			Creds:  "secretname",
		},
	},
}

var monitor = lbv1.Monitor{
	Path:        "/",
	Port:        80,
	MonitorType: "http",
}

var pool = &lbv1.Pool{
	Name: "test-pool",
	Members: []lbv1.PoolMember{{
		Node: lbv1.Node{
			Name:   "test-node-1",
			Host:   testNodeIP,
			Labels: map[string]string{"node-role.kubernetes.io/master": ""},
		},
		Port: 80},
		{
			Node: lbv1.Node{
				Name:   "test-node-2",
				Host:   "1.1.1.2",
				Labels: map[string]string{"node-role.kubernetes.io/master": ""},
			},
			Port: 80},
	},
}

var VIP = &lbv1.VIP{
	Name: "test-vip",
	Pool: pool.Name,
	IP:   dummyHostIP,
}

var _ = Describe("Controllers/Backend/controller/backend_controller", func() {

	Context("When using a creating backends", func() {
		var ctx = context.TODO()

		It("Should validate that backend provider is registered", func() {
			Expect(ListProviders()).Should(ContainElement("dummy"))
		})

		It("Should return error if backend provider tries to register again", func() {
			err := RegisterProvider("Dummy", new(d.DummyProvider))
			Expect(err).To(MatchError(MatchRegexp("provider already exists.*")))
		})

		It("Should return error if backend provider does not exist", func() {
			loadBalancer := &lbv1.ExternalLoadBalancer{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "dummy-backend",
					Namespace: "default",
				},
				Spec: lbv1.ExternalLoadBalancerSpec{
					Vip: "10.0.0.1",
					Provider: lbv1.Provider{
						Vendor: "unknown",
						Host:   dummyHostIP,
						Port:   443,
						Creds:  "secretname",
					},
				},
			}
			createdBackend, err := CreateBackend(ctx, &loadBalancer.Spec.Provider, "username", "password")
			Expect(err).Should(HaveOccurred())
			Expect(err).To(MatchError(MatchRegexp("no such provider.*")))
			Expect(createdBackend).To(BeNil())
		})

		It("Should create a provider with registered backend provider", func() {
			createdBackend, err := CreateBackend(ctx, &loadBalancer.Spec.Provider, "username", "password")
			Expect(err).ShouldNot(HaveOccurred())
			Expect(reflect.TypeOf(createdBackend.Provider)).Should(Equal(reflect.TypeOf(&d.DummyProvider{})))
		})

		It("Should handle a provider monitor", func() {
			createdBackend, err := CreateBackend(ctx, &loadBalancer.Spec.Provider, "username", "password")
			Expect(err).ShouldNot(HaveOccurred())
			err = createdBackend.HandleMonitors(ctx, &monitor)
			Expect(err).ShouldNot(HaveOccurred())
		})

		It("Should handle a provider pool", func() {
			createdBackend, err := CreateBackend(ctx, &loadBalancer.Spec.Provider, "username", "password")
			Expect(err).ShouldNot(HaveOccurred())
			err, requeueAfter, _ := createdBackend.HandlePool(ctx, pool, &monitor, loadBalancer, []lbv1.DrainingMember{})
			Expect(err).ShouldNot(HaveOccurred())
			Expect(requeueAfter).Should(Equal(0))
		})

		It("Should handle a provider VIP", func() {
			createdBackend, err := CreateBackend(ctx, &loadBalancer.Spec.Provider, "username", "password")
			Expect(err).ShouldNot(HaveOccurred())
			err = createdBackend.HandleVIP(ctx, VIP)
			Expect(err).ShouldNot(HaveOccurred())
		})

		It("Should handle a provider cleanup", func() {
			createdBackend, err := CreateBackend(ctx, &loadBalancer.Spec.Provider, "username", "password")
			Expect(err).ShouldNot(HaveOccurred())
			err = createdBackend.HandleCleanup(ctx, loadBalancer)
			Expect(err).ShouldNot(HaveOccurred())
		})

	})
	Context("When using auxiliary functions", func() {
		It("Should return true if array contains member", func() {
			m := lbv1.PoolMember{
				Node: lbv1.Node{
					Name: "node1",
					Host: testNodeIP,
				},
				Port: 80,
			}
			a := []lbv1.PoolMember{m}
			output := ContainsMember(a, m)
			Expect(output).To(BeTrue())
		})

		It("Should return false if array doesn't contain member", func() {
			m := lbv1.PoolMember{
				Node: lbv1.Node{
					Name: "node1",
					Host: testNodeIP,
				},
				Port: 80,
			}
			m2 := m.DeepCopy()
			m2.Node.Host = "1.1.1.2"
			a := []lbv1.PoolMember{m}
			output := ContainsMember(a, *m2)
			Expect(output).To(BeFalse())
		})
	})

	Context("When using drain helper functions", func() {
		var testMember lbv1.PoolMember
		var testDrainingMember lbv1.DrainingMember
		var drainingList []lbv1.DrainingMember

		BeforeEach(func() {
			testMember = lbv1.PoolMember{
				Node: lbv1.Node{
					Name: "test-node",
					Host: "10.0.1.100",
				},
				Port: 6443,
			}

			testDrainingMember = lbv1.DrainingMember{
				PoolName:  "test-pool",
				Node:      testMember.Node,
				Port:      testMember.Port,
				StartTime: metav1.Now(),
			}

			drainingList = []lbv1.DrainingMember{testDrainingMember}
		})

		Describe("IsMemberDraining", func() {
			It("Should return nil when member is not in draining list", func() {
				emptyList := []lbv1.DrainingMember{}
				result := IsMemberDraining(emptyList, &testMember, "test-pool")
				Expect(result).To(BeNil())
			})

			It("Should return nil when member host does not match", func() {
				differentMember := testMember
				differentMember.Node.Host = "10.0.1.200"
				result := IsMemberDraining(drainingList, &differentMember, "test-pool")
				Expect(result).To(BeNil())
			})

			It("Should return nil when member port does not match", func() {
				differentMember := testMember
				differentMember.Port = 8443
				result := IsMemberDraining(drainingList, &differentMember, "test-pool")
				Expect(result).To(BeNil())
			})

			It("Should return nil when pool name does not match", func() {
				result := IsMemberDraining(drainingList, &testMember, "different-pool")
				Expect(result).To(BeNil())
			})

			It("Should return draining member when all criteria match", func() {
				result := IsMemberDraining(drainingList, &testMember, "test-pool")
				Expect(result).NotTo(BeNil())
				Expect(result.PoolName).To(Equal("test-pool"))
				Expect(result.Node.Host).To(Equal("10.0.1.100"))
				Expect(result.Port).To(Equal(6443))
			})

			It("Should find correct member in list with multiple draining members", func() {
				member2 := lbv1.PoolMember{
					Node: lbv1.Node{Name: "node2", Host: "10.0.1.101"},
					Port: 6443,
				}
				draining2 := lbv1.DrainingMember{
					PoolName:  "test-pool",
					Node:      member2.Node,
					Port:      member2.Port,
					StartTime: metav1.Now(),
				}
				multiList := append(drainingList, draining2)

				result := IsMemberDraining(multiList, &member2, "test-pool")
				Expect(result).NotTo(BeNil())
				Expect(result.Node.Host).To(Equal("10.0.1.101"))
			})
		})

		Describe("RemoveDrainingMember", func() {
			It("Should return empty list when removing only member", func() {
				result := RemoveDrainingMember(drainingList, &testMember, "test-pool")
				Expect(result).To(BeEmpty())
			})

			It("Should return unchanged list when member not found", func() {
				differentMember := testMember
				differentMember.Node.Host = "10.0.1.200"
				result := RemoveDrainingMember(drainingList, &differentMember, "test-pool")
				Expect(result).To(HaveLen(1))
				Expect(result[0].Node.Host).To(Equal("10.0.1.100"))
			})

			It("Should remove only matching member from list", func() {
				member2 := lbv1.PoolMember{
					Node: lbv1.Node{Name: "node2", Host: "10.0.1.101"},
					Port: 6443,
				}
				draining2 := lbv1.DrainingMember{
					PoolName:  "test-pool",
					Node:      member2.Node,
					Port:      member2.Port,
					StartTime: metav1.Now(),
				}
				multiList := append(drainingList, draining2)

				result := RemoveDrainingMember(multiList, &testMember, "test-pool")
				Expect(result).To(HaveLen(1))
				Expect(result[0].Node.Host).To(Equal("10.0.1.101"))
			})

			It("Should preserve members from different pools", func() {
				differentPoolMember := lbv1.DrainingMember{
					PoolName:  "different-pool",
					Node:      testMember.Node,
					Port:      testMember.Port,
					StartTime: metav1.Now(),
				}
				multiList := append(drainingList, differentPoolMember)

				result := RemoveDrainingMember(multiList, &testMember, "test-pool")
				Expect(result).To(HaveLen(1))
				Expect(result[0].PoolName).To(Equal("different-pool"))
			})
		})
	})

	Context("When using drain orchestration in HandlePool", func() {
		var ctx context.Context
		var testPool *lbv1.Pool
		var testMonitor *lbv1.Monitor
		var lbWithDrainEnabled *lbv1.ExternalLoadBalancer
		var lbWithDrainDisabled *lbv1.ExternalLoadBalancer

		BeforeEach(func() {
			ctx = context.TODO()

			testPool = &lbv1.Pool{
				Name: "test-pool-drain",
				Members: []lbv1.PoolMember{{
					Node: lbv1.Node{
						Name: "test-node-1",
						Host: "10.0.1.100",
					},
					Port: 6443,
				}},
			}

			testMonitor = &lbv1.Monitor{
				Path:        "/healthz",
				Port:        6443,
				MonitorType: "https",
			}

			lbWithDrainEnabled = &lbv1.ExternalLoadBalancer{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "lb-drain-enabled",
					Namespace: "default",
				},
				Spec: lbv1.ExternalLoadBalancerSpec{
					Vip: "192.168.1.100",
					Provider: lbv1.Provider{
						Vendor: "Dummy",
						Host:   "1.2.3.4",
						Port:   443,
						Creds:  "secretname",
					},
					Drain: &lbv1.DrainConfig{
						Enabled:        true,
						TimeoutSeconds: 30,
					},
				},
			}

			lbWithDrainDisabled = &lbv1.ExternalLoadBalancer{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "lb-drain-disabled",
					Namespace: "default",
				},
				Spec: lbv1.ExternalLoadBalancerSpec{
					Vip: "192.168.1.101",
					Provider: lbv1.Provider{
						Vendor: "Dummy",
						Host:   "1.2.3.4",
						Port:   443,
						Creds:  "secretname",
					},
					Drain: &lbv1.DrainConfig{
						Enabled:        false,
						TimeoutSeconds: 30,
					},
				},
			}
		})

		It("Should return zero requeue time when drain is disabled", func() {
			backend, err := CreateBackend(ctx, &lbWithDrainDisabled.Spec.Provider, "username", "password")
			Expect(err).Should(BeNil())

			err, requeueAfter, drainingMembers := backend.HandlePool(ctx, testPool, testMonitor, lbWithDrainDisabled, []lbv1.DrainingMember{})
			Expect(err).Should(BeNil())
			Expect(requeueAfter).Should(Equal(0))
			Expect(drainingMembers).Should(BeEmpty())
		})

		It("Should return zero requeue time when drain config is nil", func() {
			lbNoDrain := lbWithDrainEnabled.DeepCopy()
			lbNoDrain.Spec.Drain = nil

			backend, err := CreateBackend(ctx, &lbNoDrain.Spec.Provider, "username", "password")
			Expect(err).Should(BeNil())

			err, requeueAfter, drainingMembers := backend.HandlePool(ctx, testPool, testMonitor, lbNoDrain, []lbv1.DrainingMember{})
			Expect(err).Should(BeNil())
			Expect(requeueAfter).Should(Equal(0))
			Expect(drainingMembers).Should(BeEmpty())
		})

		It("Should handle empty pool with drain enabled", func() {
			backend, err := CreateBackend(ctx, &lbWithDrainEnabled.Spec.Provider, "username", "password")
			Expect(err).Should(BeNil())

			emptyPool := &lbv1.Pool{
				Name:    "empty-pool",
				Members: []lbv1.PoolMember{},
			}

			err, requeueAfter, drainingMembers := backend.HandlePool(ctx, emptyPool, testMonitor, lbWithDrainEnabled, []lbv1.DrainingMember{})
			Expect(err).Should(BeNil())
			Expect(requeueAfter).Should(Equal(0))
			Expect(drainingMembers).Should(BeEmpty())
		})

		// Note: Testing the full drain orchestration (disable -> wait -> delete) requires
		// mocking the provider methods or using integration tests with actual load balancers.
		// The Dummy provider logs operations but doesn't maintain state, so we can verify
		// the orchestration logic calls the right methods but cannot easily test the
		// time-based transitions without more complex mocking or integration tests.
	})
})
