package kruise

// Labels and Annotations for CloneSet
const (
	LabelCloneSetMode = "cloneset.asi/mode"
	CloneSetASI       = "asi"

	LabelCloneSetDisableInjectSn         = "cloneset.asi/disable-inject-sn"
	AnnotationCloneSetUsePodGenerateName = "cloneset.asi/use-pod-generate-name"
	AnnotationCloneSetUpgradeTimeout     = "cloneset.asi/upgrade-timeout"

	AnnotationCloneSetBatchAdoptionToAbandon = "cloneset.asi/batchadoption-toabandon"
	AnnotationCloneSetBatchAdoptionToAdopt   = "cloneset.asi/batchadoption-toadopt"

	AnnotationCloneSetAppRulesUpdateRequired = "cloneset.asi/apprules-update-required"
)

// Labels and Annotations for proxy
const (
	AnnotationCloneSetProxyInitializing = "cloneset.asi/proxy-initializing"

	// Generation of the source workload
	AnnotationCloneSetProxyGeneration = "cloneset.asi/proxy-generation"
)
