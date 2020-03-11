package main

import (
	"errors"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/MEDIGO/go-healthz"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/sparrc/go-ping"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

func getpodlocallist() []string {
	var kubeconfig *string
	if home := homeDir(); home != "" {
		kubeconfig = flag.String("kubeconfig", filepath.Join(home, ".kube", "config"), "(optional) absolute path to the kubeconfig file")
	} else {
		kubeconfig = flag.String("kubeconfig", "", "absolute path to the kubeconfig file")
	}
	flag.Parse()
	config, err := clientcmd.BuildConfigFromFlags("", *kubeconfig)
	if err != nil {
		panic(err.Error())
	}
	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		panic(err.Error())
	}
	for {
		targets := []string{}
		pods, err := clientset.CoreV1().Pods("").List(metav1.ListOptions{})
		if err != nil {
			panic(err.Error())
		}
		for _, pod := range pods.Items {
			if pod.Status.HostIP != pod.Status.PodIP {
				//fmt.Println(pod.Name, pod.Status.PodIP)
				targets = append(targets, pod.Status.PodIP)
			}
		}
		return targets
	}
}
func getpodlist() []string {
	config, err := rest.InClusterConfig()
	if err != nil {
		panic(err.Error())
	}
	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		panic(err.Error())
	}
	for {
		targets := []string{}
		pods, err := clientset.CoreV1().Pods("").List(metav1.ListOptions{})
		if err != nil {
			panic(err.Error())
		}
		for _, pod := range pods.Items {
			if pod.Status.HostIP != pod.Status.PodIP {
				//fmt.Println(pod.Name, pod.Status.PodIP)
				targets = append(targets, pod.Status.PodIP)
			}
		}
		return targets
	}
}
func main() {
	go serve()
	go health()
	gauge := prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "ping_average_latency_k8s_pods_millisec",
		Help: "latency of targets",
	}, []string{"target"})
	prometheus.MustRegister(gauge)
	time.Sleep(1)
	hosts := getpodlist()

	for {
		for _, host := range hosts {
			ip := host
			pinger, err := ping.NewPinger(ip)
			if err != nil {
				panic(err)
			}
			pinger.Count = 5
			//pinger.Interval = 100
			//pinger.Timeout = 10
			pinger.SetPrivileged(true)
			pinger.OnFinish = func(stats *ping.Statistics) {
				latencyEND := float64(stats.AvgRtt)
				fmt.Printf("Pinging ip address:%v Average Rtt for 5 pings:%f ms\n", stats.Addr, latencyEND)
				gauge.With(prometheus.Labels{"target": stats.Addr}).Set(latencyEND)
			}
			go pinger.Run()
			time.Sleep(750 * time.Millisecond)
		}
	}
}
func serve() {
	fmt.Printf("listening on 9101 ,at /metrics\n")
	http.Handle("/metrics", promhttp.Handler())
	log.Fatal(http.ListenAndServe(":9101", nil))
}
func health() {
	var version = "1.0.0"
	healthz.Set("version", version)

	healthz.Register("Check_Metrics_Export", time.Second*10, func() error {
		_, err := http.Get("http://127.0.0.1:9101/metrics")
		if err != nil {
			return errors.New("Service Unavailable")
		}
		return nil
	})
	http.Handle("/healthz", healthz.Handler())
	fmt.Printf("listening on 8000 ,at /healthz\n")
	http.ListenAndServe(":8000", nil)

}

func homeDir() string {
	h := os.Getenv("HOME")
	return h
}
