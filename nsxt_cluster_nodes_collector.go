package main

import (
	"crypto/tls"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"net/http/httputil"
	"os"
	"strconv"
	"time"
	// Prometheus client library
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// ClusterStatusResponse definitions
type ClusterStatusResponse struct {
	ClusterID            string               `json:"cluster_id,omitempty"`
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
	ClusterInitNodeInfo []ClusterInitNodeInfo `json:"required_members_for_initialization,omitempty"`
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
	NodeStatusProperties  NodeStatusProperties  `json:"system_status"`
	NodeMgmtClusterStatus NodeMgmtClusterStatus `json:"mgmt_cluster_status"`
	NodeVersion           string                `json:"version"`
}

type NodeStatusProperties struct {
	NodeMemUsed     int32            `json:"mem_used"`
	SysTime         int32            `json:"system_time"`
	NodeFileStatus  []NodeFileStatus `json:"file_systems"`
	NodeLoadAverage []float64        `json:"load_averages"`
	NodeSwapTotal   int32            `json:"swap_total"`
	NodeMemCache    int32            `json:"mem_cache"`
	NodeCPUs        int32            `json:"cpu_cores"`
	NodeSource      string           `json:"source"`
	NodeMemTotal    int32            `json:"mem_total"`
	NodeSwapUsed    int32            `json:"swap_used"`
	NodeUpTime      int32            `json:"uptime"`
}

type NodeFileStatus struct {
	FileSystemName string `json:"file_system"`
	DiskTotal      int32  `json:"total"`
	DiskUsed       int32  `json:"used"`
	FileSystemType string `json:"type"`
	MountName      string `json:"mount"`
}

type NodeMgmtClusterStatus struct {
	MgmtClusterStatus string `json:"mgmt_cluster_status"`
}

// EdgeClusterResponse and Node Definitions
type EdgeClustersResponse struct {
	Cursor       string         `json:"cursor"`
	ResultCount  int16          `json:"result_count"`
	EdgeClusters []EdgeClusters `json:"results"`
}

type EdgeClusters struct {
	Revision               string                 `json:"_revision"`
	Id                     string                 `json:"is"`
	ResourceType           string                 `json:"resource_type"`
	ClusterProfileBindings ClusterProfileBindings `json:"cluster_profile_bindings"`
	EdgeNoodes             []EdgeNodes            `json:"members"`
	SystemOwned            string                 `json:"_system_owned"`
	DeploymentType         string                 `json:"deployment_type"`
	LastModifiedUser       string                 `json:"_last_modified_user"`
	LastModifiedTime       int32                  `json:"_last_modified_time"`
	CreateTime             int32                  `json:"_create_time`
	CreateUser             string                 `json:"_create_user`
}

type ClusterProfileBindings struct {
	ResourceType string `json:"resource_type"`
	ProfileID    string `json:"profile_id"`
}

type EdgeNodes struct {
	Id    string `json:"transport_node_id"`
	Index string `json:"member_index"`
}

type EdgeClusterStatus struct {
	Id             string           `json:"edge_cluster_id"`
	EdgeNodeStatus []EdgeNodeStatus `json:"member_status"`
	LastUpdateTime int32            `json:"last_update_timestamp"`
	Status         string           `json:"edge_cluster_status"`
}

type EdgeNodeStatus struct {
	Id     string `json:"transport_node_id"`
	Status string `json:"status"`
}

// IP Pools and Blocks Definitions
type IPPoolsResponse struct {
	Cursor        string    `json:"cursor"`
	SortBy        string    `json:"sort_by"`
	SortAscending bool      `json:"sort_ascending"`
	IPPools       []IPPools `json:"results"`
}

type IPPools struct {
	Revision         string    `json:"_revision"`
	Id               string    `json:"id"`
	Name             string    `json:"display_name"`
	Description      string    `json:"description"`
	ResourceType     string    `json:"description"`
	Subnets          []subnets `json:"description"`
	LastModifiedUser string    `json:"_last_modified_user"`
	LastModifiedTime int32     `json:"_last_modified_time"`
	CreateTime       int32     `json:"_create_time`
	CreateUser       string    `json:"_create_user`
}

type subnets struct {
	DnsNameservers   []string           `json:"dns_nameservers"`
	AllocationRanges []AllocationRanges `json:"allocation_ranges"`
	GatewayIP        string             `json:"gateway_ip"`
	Cidr             string             `json:"cidr"`
}

type AllocationRanges struct {
	StartIP string `json:"start"`
	EndIP   string `json:"end"`
}

type IPPoolAllocations struct {
	Count       int32    `json:"result_count"`
	AllocatedIP []string `json:"allocation_id"`
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
var mgmtClusterNodeCPULoadMetric = prometheus.NewGaugeVec(
	prometheus.GaugeOpts{
		Name: "mgmt_cluster_node_cpu",
		Help: "Shows the cpu % for the mgmt cluster node",
	},
	[]string{
		"nsxt_host",
		"mgmt_cluster_id",
		"node_id",
		"processor_id",
	},
)
var mgmtClusterNodeMemUsedMetric = prometheus.NewGaugeVec(
	prometheus.GaugeOpts{
		Name: "mgmt_cluster_node_memory",
		Help: "Shows the memory usage for the mgmt cluster node",
	},
	[]string{
		"nsxt_host",
		"mgmt_cluster_id",
		"node_id",
	},
)
var mgmtClusterNodeMemTotalMetric = prometheus.NewGaugeVec(
	prometheus.GaugeOpts{
		Name: "mgmt_cluster_node_memory_total",
		Help: "Shows the total memory  for the mgmt cluster node",
	},
	[]string{
		"nsxt_host",
		"mgmt_cluster_id",
		"node_id",
	},
)
var mgmtClusterNodeMemCacheMetric = prometheus.NewGaugeVec(
	prometheus.GaugeOpts{
		Name: "mgmt_cluster_node_memory_cache",
		Help: "Shows the total memory cache for the mgmt cluster node",
	},
	[]string{
		"nsxt_host",
		"mgmt_cluster_id",
		"node_id",
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
func getNsxClusterStatus(interval int) {

	go func() {
		for {
			t1 := time.Now()
			var client http.Client

			http.DefaultTransport.(*http.Transport).TLSClientConfig = &tls.Config{InsecureSkipVerify: true}

			req, err := http.NewRequest("GET", "https://"+nsxthost+"/api/v1/cluster/status", nil)
			//req, err := http.NewRequest("GET","http://" + nsxthost + "/api/v1/cluster/status",nil)
			req.Header.Add("Authorization", "Basic "+basicAuth(apiuser, apipass))

			if err != nil {
				log.Fatal(err)
			}

			// Adding Request Dump
			if debug {
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
			if debug {
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
			if err != nil {
				log.Fatal(marsherr)
			}

			clusterID = responseObject.ClusterID
			if debug {
				log.Println("Mgmt Cluster Status: " + responseObject.MgmtClusterStatus.Status)
				log.Println("Mgmt Cluster ID: " + clusterID)
			}
			mgmtClusterStatusMetric.WithLabelValues(clusterID, responseObject.MgmtClusterStatus.Status).Set(1.0)

			if debug {
				log.Println("Controller Cluster Status" + responseObject.ControlClusterStatus.Status)
			}
			controllerClusterStatusMetric.WithLabelValues(clusterID, responseObject.ControlClusterStatus.Status).Set(1.0)
			for i := 0; i < len(responseObject.MgmtClusterStatus.OnlineNodes); i++ {
				if debug {
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
			time.Sleep(time.Duration(interval) * time.Second)
		}
	}()

}

func getNsxClusterNodeMetrics(id string, clusterid string) {

	t1 := time.Now()
	var client http.Client

	http.DefaultTransport.(*http.Transport).TLSClientConfig = &tls.Config{InsecureSkipVerify: true}

	req, err := http.NewRequest("GET", "https://"+nsxthost+"/api/v1/cluster/nodes/"+id+"/status", nil)
	//req, err := http.NewRequest("GET","http://" + nsxthost + "/api/v1/cluster/nodes/" + id + "/status",nil)
	req.Header.Add("Authorization", "Basic "+basicAuth(apiuser, apipass))

	if err != nil {
		log.Fatal(err)
	}

	// Adding Request Dump
	if debug {
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
	if debug {
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

	/*type NodeStatusProperties struct {
	  	NodeMemUsed         int32            `json:"mem_used"`
	  	SysTime         int32            `json:"system_time"`
	  	NodeFileStatus  []NodeFileStatus `json:"file_systems"`
	  	NodeLoadAverage []float64        `json:"load_averages"`
	  	NodeSwapTotal   int32            `json:"swap_total"`
	  	NodeMemCache    int32            `json:"mem_cache"`
	  	NodeCPUs        int32            `json:"cpu_cores"`
	  	NodeSource      string           `json:"source"`
	  	NodeMemTotal    int32            `json:"mem_total"`
	  	NodeSwapUsed    int32            `json:"swap_used"`
	  	NodeUpTime      int32            `json:"uptime"`
	  }
	*/

	nodeClusterStatus := responseObject.NodeMgmtClusterStatus.MgmtClusterStatus
	nodeCPULoad := responseObject.NodeStatusProperties.NodeLoadAverage
	nodeMemUsed := responseObject.NodeStatusProperties.NodeMemUsed
	nodeMemTotal := responseObject.NodeStatusProperties.NodeMemTotal
	nodeMemCache := responseObject.NodeStatusProperties.NodeMemCache

	mgmtClusterNodeMemUsedMetric.WithLabelValues(nsxthost, clusterid, id).Set(float64(nodeMemUsed))
	mgmtClusterNodeMemTotalMetric.WithLabelValues(nsxthost, clusterid, id).Set(float64(nodeMemTotal))
	mgmtClusterNodeMemCacheMetric.WithLabelValues(nsxthost, clusterid, id).Set(float64(nodeMemCache))
	for i := 0; i <= len(nodeCPULoad); i++ {
		mgmtClusterNodeCPULoadMetric.WithLabelValues(nsxthost, clusterid, id, strconv.Itoa(i)).Set(nodeCPULoad[i])
	}

	if debug {
		log.Println("Controller cluster node status:" + nodeClusterStatus)
		log.Printf("Mgmt cluster node memory used: %d total: %d cached %d\n", nodeMemUsed, nodeMemTotal, nodeMemCache)
		log.Printf("Mgmt cluster node cpu load: %v\n", nodeCPULoad)
	}

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
var envinterval string
var clusterStatusInterval int
var err error

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

	envinterval = os.Getenv("CLUSTER_STATUS_INTERVAL")
	clusterStatusInterval, err = strconv.Atoi(envinterval)

	if err == nil && clusterStatusInterval > 0 {
		fmt.Println("Interval for cluster status set to " + envinterval + " secs")
	} else {
		fmt.Println("Using default metric interval of 60 secs")
		clusterStatusInterval = 60
	}

	//Create metric registrations and handler for Prometheus
	r := prometheus.NewRegistry()

	r.MustRegister(controllerClusterStatusMetric)
	r.MustRegister(mgmtClusterStatusMetric)
	r.MustRegister(mgmtClusterNodeStatusMetric)
	r.MustRegister(mgmtClusterNodeCPULoadMetric)
	r.MustRegister(mgmtClusterNodeMemUsedMetric)
	r.MustRegister(mgmtClusterNodeMemTotalMetric)
	r.MustRegister(mgmtClusterNodeMemCacheMetric)
	r.MustRegister(edgeClusterStatusMetric)
	r.MustRegister(edgeClusterNodeStatusMetric)

	// Start Controller Cluster Metric Collection
	getNsxClusterStatus(clusterStatusInterval)

	//This section will start the HTTP server and expose
	//any metrics on the /metrics endpoint.
	handler := promhttp.HandlerFor(r, promhttp.HandlerOpts{})
	http.Handle("/metrics", handler)
	if debug {
		log.Println("Beginning to serve on port :8080")
	}
	if debug {
		log.Fatal(http.ListenAndServe(":8080", nil))
	}
}
