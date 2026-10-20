// Package etcdcoord assembles the etcd infrastructure for the ID generation domain:
// lease lifecycle and worker slot coordination are provided by the shared
// devops-infra/etcd library; this package builds TagInfra per biz_tag and
// adapts all infrastructure errors into domain errors.
package etcdcoord

import (
	iddomain "github.com/stanley760/devops/devops-id/internal/domain/idgen"
	"github.com/stanley760/devops/devops-id/internal/infrastructure/workerfile"
	etcdinfra "github.com/stanley760/devops/devops-infra/etcd"
	"time"
)

// NewInfraFactory returns a factory that builds the domain TagInfra per biz_tag.
// When client is nil, Lease/Slots are nil (the domain layer runs in local mode);
// when cacheRoot is empty, it defaults to "data".
func NewInfraFactory(client *etcdinfra.Client, leafName string, leaseTTL time.Duration, cacheRoot string) iddomain.TagInfraFactory {
	if cacheRoot == "" {
		cacheRoot = "data"
	}
	return func(bizTag string) iddomain.TagInfra {
		infra := iddomain.TagInfra{Cache: workerfile.New(cacheRoot, leafName, bizTag)}
		if client != nil {
			lease := etcdinfra.NewLeaseManager(client, leaseTTL)
			infra.Lease = leaseAdapter{inner: lease}
			infra.Slots = slotCoordinatorAdapter{coord: etcdinfra.NewSlotCoordinator(client, lease)}
		}
		return infra
	}
}

// Compile-time port assertions: infrastructure implementations must satisfy the domain ports.
var (
	_ iddomain.LeaseLifecycle  = leaseAdapter{}
	_ iddomain.SlotCoordinator = slotCoordinatorAdapter{}
	_ iddomain.WorkerCache     = (*workerfile.Cache)(nil)
)
