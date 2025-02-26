// +build e2e

package scenarios

import (
	"context"
	"testing"

	. "github.com/onsi/gomega"
	e2eutil "github.com/openshift/hypershift/test/e2e/util"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	crclient "sigs.k8s.io/controller-runtime/pkg/client"

	hyperv1 "github.com/openshift/hypershift/api/v1alpha1"
	cmdcluster "github.com/openshift/hypershift/cmd/cluster"
)

type TestCreateClusterOptions struct {
	AWSCredentialsFile string
	AWSRegion          string
	PullSecretFile     string
	ReleaseImage       string
	ArtifactDir        string
	BaseDomain         string
}

// TestCreateCluster implements a test that mimics the operation described in the
// HyperShift quick start (creating a basic guest cluster).
//
// This test is meant to provide a first, fast signal to detect regression; it
// is recommended to use it as a PR blocker test.
func TestCreateCluster(ctx context.Context, o TestCreateClusterOptions) func(t *testing.T) {
	return func(t *testing.T) {
		t.Parallel()
		g := NewWithT(t)

		client := e2eutil.GetClientOrDie()

		// Create a namespace in which to place hostedclusters
		namespace := e2eutil.GenerateNamespace(t, ctx, client, "e2e-clusters-")
		name := e2eutil.SimpleNameGenerator.GenerateName("example-")

		// Define the cluster we'll be testing
		hostedCluster := &hyperv1.HostedCluster{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: namespace.Name,
				Name:      name,
			},
		}

		// Ensure we clean up after the test
		defer func() {
			// TODO: Figure out why this is slow
			//e2eutil.DumpGuestCluster(context.Background(), client, hostedCluster, o.ArtifactDir)
			e2eutil.DumpAndDestroyHostedCluster(t, context.Background(), hostedCluster, o.AWSCredentialsFile, o.AWSRegion, o.BaseDomain, o.ArtifactDir)
			e2eutil.DeleteNamespace(t, context.Background(), client, namespace.Name)
		}()

		// Create the cluster
		createClusterOpts := cmdcluster.Options{
			Namespace:          hostedCluster.Namespace,
			Name:               hostedCluster.Name,
			InfraID:            hostedCluster.Name,
			ReleaseImage:       o.ReleaseImage,
			PullSecretFile:     o.PullSecretFile,
			AWSCredentialsFile: o.AWSCredentialsFile,
			Region:             o.AWSRegion,
			// TODO: generate a key on the fly
			SSHKeyFile:       "",
			NodePoolReplicas: 2,
			InstanceType:     "m4.large",
			BaseDomain:       o.BaseDomain,
			NetworkType:      string(hyperv1.OpenShiftSDN),
		}
		t.Logf("Creating a new cluster. Options: %v", createClusterOpts)
		err := cmdcluster.CreateCluster(ctx, createClusterOpts)
		g.Expect(err).NotTo(HaveOccurred(), "failed to create cluster")

		// Get the newly created cluster
		err = client.Get(ctx, crclient.ObjectKeyFromObject(hostedCluster), hostedCluster)
		g.Expect(err).NotTo(HaveOccurred(), "failed to get hostedcluster")
		t.Logf("Found the new hostedcluster. Namespace: %s, name: %s", hostedCluster.Namespace, hostedCluster.Name)

		// Get the newly created nodepool
		nodepool := &hyperv1.NodePool{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: hostedCluster.Namespace,
				Name:      hostedCluster.Name,
			},
		}
		err = client.Get(ctx, crclient.ObjectKeyFromObject(nodepool), nodepool)
		g.Expect(err).NotTo(HaveOccurred(), "failed to get nodepool")
		t.Logf("Created nodepool. Namespace: %s, name: %s", nodepool.Namespace, nodepool.Name)

		// Wait for nodes to report ready
		guestClient := e2eutil.WaitForGuestClient(t, ctx, client, hostedCluster)
		e2eutil.WaitForNReadyNodes(t, ctx, guestClient, *nodepool.Spec.NodeCount)

		// Wait for the rollout to be reported complete
		t.Logf("Waiting for cluster rollout. Image: %s", o.ReleaseImage)
		e2eutil.WaitForImageRollout(t, ctx, client, hostedCluster, o.ReleaseImage)
	}
}
