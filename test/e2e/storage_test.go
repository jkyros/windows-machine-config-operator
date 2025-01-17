package e2e

import (
	"context"
	"log"
	"testing"

	config "github.com/openshift/api/config/v1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	core "k8s.io/api/core/v1"
	meta "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/openshift/windows-machine-config-operator/test/e2e/providers/vsphere"
)

// testStorage tests that persistent volumes can be accessed by Windows pods
func testStorage(t *testing.T) {
	tc, err := NewTestContext()
	require.NoError(t, err)
	if !tc.StorageSupport() {
		t.Skip("storage is not supported on this platform")
	}
	if inTreeUpgrade && tc.CloudProvider.GetType() != config.VSpherePlatformType {
		t.Skip("in tree upgrade is only testable on vSphere")
	}
	err = tc.waitForConfiguredWindowsNodes(gc.numberOfMachineNodes, false, false)
	require.NoError(t, err, "timed out waiting for Windows Machine nodes")
	err = tc.waitForConfiguredWindowsNodes(gc.numberOfBYOHNodes, false, true)
	require.NoError(t, err, "timed out waiting for BYOH Windows nodes")
	require.Greater(t, len(gc.allNodes()), 0, "test requires at least one Windows node to run")

	// Create the PVC and choose the node the deployment will be scheduled on. This is necessary as ReadWriteOnly
	// volumes can only be bound to a single node.
	// https://docs.openshift.com/container-platform/4.12/storage/understanding-persistent-storage.html#pv-access-modes_understanding-persistent-storage
	var pvc *core.PersistentVolumeClaim
	if inTreeUpgrade {
		vsphereProvider, ok := tc.CloudProvider.(*vsphere.Provider)
		require.True(t, ok, "in tree upgrade must be ran on vSphere")
		pvc, err = vsphereProvider.CreateInTreePVC(tc.client.K8s, tc.workloadNamespace)
		require.NoError(t, err)
	} else {
		pvc, err = tc.CloudProvider.CreatePVC(tc.client.K8s, tc.workloadNamespace)
		require.NoError(t, err)
	}
	if !skipWorkloadDeletion {
		defer func() {
			err := tc.client.K8s.CoreV1().PersistentVolumeClaims(tc.workloadNamespace).Delete(context.TODO(),
				pvc.GetName(), meta.DeleteOptions{})
			if err != nil {
				log.Printf("error deleting PVC: %s", err)
			}
		}()
	}
	pvcVolumeSource := &core.PersistentVolumeClaimVolumeSource{ClaimName: pvc.GetName()}
	affinity, err := getAffinityForNode(&gc.allNodes()[0])
	require.NoError(t, err)

	// The deployment will not come to ready if the volume is not able to be attached to the pod. If the deployment is
	// successful, storage is working as expected.
	winServerDeployment, err := tc.deployWindowsWebServer("win-webserver-storage-test", affinity, pvcVolumeSource)
	assert.NoError(t, err)
	if err == nil && !skipWorkloadDeletion {
		defer func() {
			err := tc.deleteDeployment(winServerDeployment.GetName())
			if err != nil {
				log.Printf("error deleting deployment: %s", err)
			}
		}()
	}
}
