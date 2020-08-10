package api

const (

	// EnvIsSidecar used to indicates this is a sidecar container
	EnvIsSidecar = "IS_SIDECAR"

	// EnvIgnoreReady used to indicates should ignore this container ready when generate container ready condition
	EnvIgnoreReady = "SIGMA_IGNORE_READY"

	// The 'EnvIgnoreResource' is used to indicate you should ignore the request/limit for this container in the pod when you compute your node resource
	// more details : https://yuque.antfin-inc.com/sigma.pouch/sigma3.x/vxeutm#cc6c6299
	EnvIgnoreResource = "SIGMA_IGNORE_RESOURCE"

)
