package api

type PriorityClass string

const (
	// PriorityClassSystem defines a system-level priority for pods.
	PriorityClassSystem PriorityClass = "System"
	// PriorityClassMonitoring defines a monitoring-level priority for pods.
	PriorityClassMonitoring PriorityClass = "Monitoring"
	// PriorityClassProduction defines a production-level priority for pods.
	PriorityClassProduction PriorityClass = "Production"
	// PriorityClassMostAvailable defines most-available-level priority for pods.
	PriorityClassMostAvailable PriorityClass = "MostAvailable"
	// PriorityClassPreemptible defines opportunistic-level priority for pods.
	PriorityClassPreemptible PriorityClass = "Preemptible"
	// PriorityClassNone defines a unknown priority,
	PriorityClassNone PriorityClass = ""
)
