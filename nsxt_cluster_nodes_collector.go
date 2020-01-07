package main

import (
	"crypto/tls"
	"encoding/json"
	"encoding/base64"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"net/http/httputil"
	"os"
	"time"

	// Prometheus client library
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// ClusterStatusResponse definitions
type ClusterStatusResponse struct {
	ClusterID	string	`json:"cluster_id,omitempty"`
	MgmtClusterStatus    MgmtClusterStatus    `json:"mgmt_cluster_status,omitempty"`
	ControlClusterStatus ControlClusterStatus `json:"control_cluster_status,omitempty"`
}


type ControlClusterStatus struct {
	Status string `json:"status,omitempty"`
}

type MgmtClusterStatus struct {
	Status              string                `json:"status,omitempty"`
	OfflineNodes        []MgmtNodes           `json:"offline_nodes,omitempty"`
	OnlineNodes         []MgmtNodes           `json:"online_nodes,omitempty"`
	OlusterInitNodeInfo []ClusterInitNodeInfo `json:"required_members_for_initialization,omitempty"`
}

type MgmtNodes struct {
	UUID string `json:"uuid,omitempty"`
	IP   string `json:"mgmt_cluster_listen_ip_address,omitempty"`
}

type ClusterInitNodeInfo struct {
	DiskStoreID string `json:"disk_store_id,omitempty"`
	HostAddress string `json:"host_address,omitempty"`
}

//ClusterNodeStatus Definitions
type ClusterNodeStatus struct {
	nodeSystemStatus      NodeStatusProperties  `json:"system_status"`
	nodeMgmtClusterStatus NodeMgmtClusterStatus `json:"mgmt_cluster_status"`
	nodeVersion           string                `json:"version"`
}

type NodeStatusProperties struct {
	memUsed         int32            `json:"mem_used"`
	sysTime         int32            `json:"system_time"`
	nodeFileStatus  []NodeFileStatus `json:"file_systems"`
	nodeLoadAverage []float64        `json:"load_averages"`
	nodeSwapTotal   int32            `json:"swap_total"`
	nodeMemCache    int32            `json:"mem_cache"`
	nodeCPUs        int32            `json:"cpu_cores"`
	nodeSource      string           `json:"source"`
	nodeMemTotal    int32            `json:"mem_total"`
	nodeSwapUsed    int32            `json:"swap_used"`
	nodeUpTime      int32            `json:"uptime"`
}

type NodeFileStatus struct {
	fileSystemName string `json:"file_system"`
	diskTotal      int32  `json:"total"`
	diskUsed       int32  `json:"used"`
	fileSystemType string `json:"type"`
	mountName      string `json:"mount"`
}

type NodeMgmtClusterStatus struct {
	nodeMgmtClusterStatus string `json:"mgmt_cluster_status"`
}

// EdgeClusterResponse and Node Definitions
type EdgeClustersResponse struct {
	cursor       string         `json:"cursor"`
	resultCount  int16          `json:"result_count"`
	edgeClusters []EdgeClusters `json:"results"`
}

type EdgeClusters struct {
	revision               string                 `json:"_revision"`
	id                     string                 `json:"is"`
	resourceType           string                 `json:"resource_type"`
	clusterProfileBindings ClusterProfileBindings `json:"cluster_profile_bindings"`
	edgeNoodes             []EdgeNodes            `json:"members"`
	systemOwned            string                 `json:"_system_owned"`
	deploymentType         string                 `json:"deployment_type"`
	lastModifiedUser       string                 `json:"_last_modified_user"`
	lastModifiedTime       int32                  `json:"_last_modified_time"`
	createTime             int32                  `json:"_create_time`
	createUser             string                 `json:"_create_user`
}

type ClusterProfileBindings struct {
	resourceType string `json:"resource_type"`
	profileID    string `json:"profile_id"`
}

type EdgeNodes struct {
	id    string `json:"transport_node_id"`
	index string `json:"member_index"`
}

type EdgeClusterStatus struct {
	id             string           `json:"edge_cluster_id"`
	edgeNodeStatus []EdgeNodeStatus `json:"member_status"`
	lastUpdateTime int32            `json:"last_update_timestamp"`
	status         string           `json:"edge_cluster_status"`
}

type EdgeNodeStatus struct {
	id     string `json:"transport_node_id"`
	status string `json:"status"`
}

// IP Pools and Blocks Definitions
type IPPoolsResponse struct {
	cursor        string    `json:"cursor"`
	sortBy        string    `json:"sort_by"`
	sortAscending bool      `json:"sort_ascending"`
	IPPools       []IPPools `json:"results"`
}

type IPPools struct {
	revision         string    `json:"_revision"`
	id               string    `json:"id"`
	name             string    `json:"display_name"`
	description      string    `json:"description"`
	resourceType     string    `json:"description"`
	subnets          []subnets `json:"description"`
	lastModifiedUser string    `json:"_last_modified_user"`
	lastModifiedTime int32     `json:"_last_modified_time"`
	createTime       int32     `json:"_create_time`
	createUser       string    `json:"_create_user`
}

type subnets struct {
	dnsNameservers   []string           `json:"dns_nameservers"`
	allocationRanges []AllocationRanges `json:"allocation_ranges"`
	gatewayIP        string             `json:"gateway_ip"`
	cidr             string             `json:"cidr"`
}

type AllocationRanges struct {
	startIP string `json:"start"`
	endIP   string `json:"end"`
}

type IPPoolAllocations struct {
	count       int32    `json:"result_count"`
	allocatedIP []string `json:"allocation_id"`
}

//Define the metrics we wish to expose
var controllerClusterStatusMetric = prometheus.NewGaugeVec(
	prometheus.GaugeOpts{
		Name: "controller_cluster_status",
		Help: "Shows the status for the controller cluster",
	},
	[]string{
		"cluster_id",
		"controller_cluster_status",
	},
)

var mgmtClusterStatusMetric = prometheus.NewGaugeVec(
	prometheus.GaugeOpts{
		Name: "mgmt_cluster_status",
		Help: "Shows the status for the management cluster",
	},
	[]string{
		"cluster_id",
		"mgmt_cluster_status",
	},
)

var mgmtClusterNodeStatusMetric = prometheus.NewGaugeVec(
	prometheus.GaugeOpts{
		Name: "mgmt_cluster__node_status",
		Help: "Shows the status for the management cluster node",
	},
	[]string{
		"cluster_id",
		"node_id",
		"mgmt_closter_node_status",
	},
)

var edgeClusterStatusMetric = prometheus.NewGaugeVec(
	prometheus.GaugeOpts{
		Name: "edge_cluster_status",
		Help: "Shows the status for the edge cluster",
	},
	[]string{
		"nsxt_host",
		"edge_cluster_id",
		"edge_cluster_status",
	},
)

var edgeClusterNodeStatusMetric = prometheus.NewGaugeVec(
	prometheus.GaugeOpts{
		Name: "edge_cluster_node_status",
		Help: "Shows the status for the edge cluster node",
	},
	[]string{
		"nsxt_host",
		"edge_cluster_id",
		"node_id",
		"edge_cluster_node_status",
	},
)

// Process NSX-T API Requests
func getNsxClusterStatus() {

	go func() {
		for {
			t1 := time.Now()
			var client http.Client

			http.DefaultTransport.(*http.Transport).TLSClientConfig = &tls.Config{InsecureSkipVerify: true}

			req, err := http.NewRequest("GET","https://" + nsxthost + "/api/v1/cluster/status",nil)
			//req, err := http.NewRequest("GET","http://" + nsxthost + "/api/v1/cluster/status",nil)
			req.Header.Add("Authorization","Basic " + basicAuth(apiuser,apipass))

			if err != nil {
				log.Fatal(err)
			}
		
			// Adding Request Dump
			if (debug) {
				dump, err := httputil.DumpRequest(req, true)
				if err != nil {
					log.Fatal(err)
				}

				log.Printf("%q", dump)
			} 

			//fmt.Printf("%q", dump)

			response, err := client.Do(req)

			if err != nil {
				log.Fatal(err)
				os.Exit(1)
			}

			defer response.Body.Close()
		
			// Adding Response dump
			if (debug) {
				respDump, err2 := httputil.DumpResponse(response, true)
				if err != nil {
					log.Fatal(err2)
				}
				log.Printf("%q", respDump)
			}

			responseData, err := ioutil.ReadAll(response.Body)
			if err != nil {
				log.Fatal(err)
			}

			var responseObject ClusterStatusResponse
			var clusterID string
	

			marsherr := json.Unmarshal(responseData, &responseObject)
			if  err != nil { 
				log.Fatal(marsherr)
			}

			clusterID=responseObject.ClusterID
			if (debug) {
				log.Println("Mgmt Cluster Status: " + responseObject.MgmtClusterStatus.Status)
				log.Println("Mgmt Cluster ID: " + clusterID)
			}
			mgmtClusterStatusMetric.WithLabelValues(clusterID, responseObject.MgmtClusterStatus.Status).Set(1.0)

			if (debug) {
				log.Println("Controller Cluster Status" + responseObject.ControlClusterStatus.Status)
			}
			controllerClusterStatusMetric.WithLabelValues(clusterID, responseObject.ControlClusterStatus.Status).Set(1.0)
			for i := 0; i < len(responseObject.MgmtClusterStatus.OnlineNodes); i++ {
				if (debug) {
					log.Println("Cluster Node ID: " + responseObject.MgmtClusterStatus.OnlineNodes[i].UUID)
					log.Println("Cluster Node IP: " + responseObject.MgmtClusterStatus.OnlineNodes[i].IP)
				}
				getNsxClusterNodeMetrics(responseObject.MgmtClusterStatus.OnlineNodes[i].UUID, clusterID)
			}
			for i := 0; i < len(responseObject.MgmtClusterStatus.OfflineNodes); i++ {
				log.Println("Cluster Node ID: " + responseObject.MgmtClusterStatus.OfflineNodes[i].UUID)
				log.Println("Cluster Node IP: " + responseObject.MgmtClusterStatus.OfflineNodes[i].IP)
				getNsxClusterNodeMetrics(responseObject.MgmtClusterStatus.OfflineNodes[i].UUID, clusterID)
			}
			diff := time.Now().Sub(t1)
			fmt.Println(diff)
			time.Sleep(60 * time.Second)
		}
	}()

}

func getNsxClusterNodeMetrics(id string, clusterid string) {

	t1 := time.Now()
	var client http.Client 

	http.DefaultTransport.(*http.Transport).TLSClientConfig = &tls.Config{InsecureSkipVerify: true}

	req, err := http.NewRequest("GET","https://" + nsxthost + "/api/v1/cluster/nodes/" + id + "/status",nil)
	//req, err := http.NewRequest("GET","http://" + nsxthost + "/api/v1/cluster/nodes/" + id + "/status",nil)
	req.Header.Add("Authorization","Basic " + basicAuth(apiuser,apipass))

	if err != nil {
		log.Fatal(err)
	}

	// Adding Request Dump
	if (debug) {
		dump, err := httputil.DumpRequest(req, true)
		if err != nil {
			log.Fatal(err)
		}
		log.Printf("%q", dump)
	}
	response, err := client.Do(req)

	if err != nil {
		log.Fatal(err)
		os.Exit(1)
	}

	defer response.Body.Close()

	// Adding Response dump
	if (debug) {
		respDump, err2 := httputil.DumpResponse(response, true)
		if err != nil {
			log.Fatal(err2)
		}
		log.Printf("%q", respDump)
		}

	responseData, err := ioutil.ReadAll(response.Body)
	if err != nil {
		log.Fatal(err)
	}

	var responseObject ClusterNodeStatus
	json.Unmarshal(responseData, &responseObject)

	if (debug) { log.Println("Controller cluster node status:" + responseObject.nodeMgmtClusterStatus.nodeMgmtClusterStatus)}
	mgmtClusterNodeStatusMetric.WithLabelValues(clusterid, id, responseObject.nodeMgmtClusterStatus.nodeMgmtClusterStatus).Set(1.0)

	diff := time.Now().Sub(t1)
	fmt.Println(diff)

}

func basicAuth(username, password string) string {
	auth := username + ":" + password
	return base64.StdEncoding.EncodeToString([]byte(auth))
}


// NSXHOST must be exported to an environment variable
var nsxthost string
var apiuser string
var apipass string
var envdebug string
var debug bool

func main() {
	nsxthost = os.Getenv("NSXTHOST")
	if len(nsxthost) == 0 {
		fmt.Println("Empty environment variable NSXTHOST. Exit..")
		os.Exit(1)
	} else {
		fmt.Println("NSXTHOST: " + nsxthost)
	}

	apiuser = os.Getenv("NSXTUSER")
	if len(apiuser) == 0 {
		fmt.Println("Empty environment variable NSXTUSER. Exit..")
		os.Exit(1)
	} else {
		fmt.Println("NSXTUSER: " + apiuser)
	}

	apipass = os.Getenv("NSXTPASS")
	if len(apipass) == 0 {
		fmt.Println("Empty environment variable NSXTPASS. Exit..")
		os.Exit(1)
	} else {
		fmt.Println("NSXTPASS: ****")
	}

	envdebug = os.Getenv("DEBUG")
	if len(envdebug) == 0 {
		debug = false
		fmt.Println("DEBUG turned off.")
	} else {
		debug = true
		fmt.Println("DEBUG turned on. Will logg requests and responses")
	}

	//Create metric registrations and handler for Prometheus
	r := prometheus.NewRegistry()

	r.MustRegister(controllerClusterStatusMetric)
	r.MustRegister(mgmtClusterStatusMetric)
	r.MustRegister(mgmtClusterNodeStatusMetric)
	r.MustRegister(edgeClusterStatusMetric)
	r.MustRegister(edgeClusterNodeStatusMetric)

	// Start Controller Cluster Metric Collection
	getNsxClusterStatus()

	//This section will start the HTTP server and expose
	//any metrics on the /metrics endpoint.
	handler := promhttp.HandlerFor(r, promhttp.HandlerOpts{})
	http.Handle("/metrics", handler)
	if (debug) { log.Println("Beginning to serve on port :8080")}
	if (debug) {log.Fatal(http.ListenAndServe(":8080", nil))}
}
