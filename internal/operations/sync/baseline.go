package syncops

import (
	"context"

	syncbaseline "github.com/JekYUlll/Dipole/internal/operations/sync/baseline"
	syncbaselinemysql "github.com/JekYUlll/Dipole/internal/operations/sync/baseline/mysql"
)

func RunSyncBaselineCapture(ctx context.Context, jobName string) (syncbaseline.Manifest, error) {
	db, store, err := openSyncRecoveryStore(ctx, "baseline capture")
	if err != nil {
		return syncbaseline.Manifest{}, err
	}
	defer db.Close()
	baseline, err := syncbaselinemysql.NewSyncBaselineStore(store)
	if err != nil {
		return syncbaseline.Manifest{}, err
	}
	return baseline.Capture(ctx, jobName)
}

func RunSyncBaselineReconciliation(ctx context.Context, jobName string, maxExamples int) (syncbaseline.Report, error) {
	db, store, err := openSyncRecoveryStore(ctx, "baseline reconciliation")
	if err != nil {
		return syncbaseline.Report{}, err
	}
	defer db.Close()
	baseline, err := syncbaselinemysql.NewSyncBaselineStore(store)
	if err != nil {
		return syncbaseline.Report{}, err
	}
	return baseline.Reconcile(ctx, jobName, maxExamples)
}

func RunSyncBaselineRestore(ctx context.Context, jobName string, maxExamples int) (syncbaseline.Report, error) {
	db, store, err := openSyncRecoveryStore(ctx, "baseline restore")
	if err != nil {
		return syncbaseline.Report{}, err
	}
	defer db.Close()
	baseline, err := syncbaselinemysql.NewSyncBaselineStore(store)
	if err != nil {
		return syncbaseline.Report{}, err
	}
	return baseline.Restore(ctx, jobName, maxExamples)
}
