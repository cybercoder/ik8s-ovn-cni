package net_utils

type IpAssignmentRequestBody struct {
	Namespace          string `json:"namespace"`
	Name               string `json:"name"`
	ContainerInterface string `json:"containerInterface"`
	IpFamily           string `json:"ipFamily"`
}

type IpAssignmentResponseBody struct {
	PublicIpPoolName   string `json:"publicIpPoolName"`
	ContainerInterface string `json:"containerInterface"`
	IpFamily           string `json:"ipFamily"`
	Address            string `json:"address"`
	MacAddress         string `json:"macAddress"`
	ResourceKind       string `json:"resourceKind"`
	ResourceNamespace  string `json:"resourceNamespace"`
	ResourceName       string `json:"resourceName"`
}
