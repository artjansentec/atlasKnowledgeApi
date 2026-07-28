package service

// MnemosProjectSyncer is notified after knowledge-changing mutations.
type MnemosProjectSyncer interface {
	NotifyProjectChanged(projectID string)
}

type noopMnemosSyncer struct{}

func (noopMnemosSyncer) NotifyProjectChanged(string) {}

func notifyMnemos(syncer MnemosProjectSyncer, projectID string) {
	if syncer == nil || projectID == "" {
		return
	}
	syncer.NotifyProjectChanged(projectID)
}
