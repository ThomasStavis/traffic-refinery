package main

import (
	"flag"
	"time"

	"github.com/traffic-refinery/traffic-refinery/internal/config"
	"github.com/traffic-refinery/traffic-refinery/internal/counters"
	"github.com/traffic-refinery/traffic-refinery/internal/flowstats"
	"github.com/traffic-refinery/traffic-refinery/internal/network"
	"github.com/traffic-refinery/traffic-refinery/internal/servicemap"
)

func BenchmarkCPU(traceFile, conf, folder string) float64 {
	var err error

	// Prepare the service map and cache
	c := config.TrafficRefineryConfig{}
	c.ImportConfigFromFile(conf)
	smapServices := []servicemap.Service{}
	fcacheServices := []flowstats.Service{}
	for i, s := range c.Services {
		smapServices = append(smapServices, servicemap.Service{
			Name: s.Name,
			ServiceFilter: servicemap.Filter{
				DomainsString: s.Filter.DomainsString,
				DomainsRegex:  s.Filter.DomainsRegex,
				Prefixes:      s.Filter.Prefixes,
			},
			Code: servicemap.ServiceID(i),
		})
		fcacheServices = append(fcacheServices, flowstats.Service{
			Name:    s.Name,
			Collect: s.Collect,
		})
	}

	var smap *servicemap.ServiceMap
	if smap, err = servicemap.NewServiceMap(c.DNSCache.EvictTime, c.DNSCache.CleanupTime); err == nil {
		panic(err)
	}
	smap.ConfigServiceMap(smapServices)

	slist := []string{}
	for _, service := range fcacheServices {
		slist = append(slist, service.Name)
	}
	counters := counters.AvailableCounters{}
	nameToID, err := counters.Build(slist)
	if err != nil {
		panic("configuration error")
	}

	serviceIdToCountersId := make(map[servicemap.ServiceID][]int)

	for _, service := range fcacheServices {
		if id, ok := smap.GetId(service.Name); ok {
			serviceIdToCountersId[id] = []int{}
			for _, c := range service.Collect {
				serviceIdToCountersId[id] = append(serviceIdToCountersId[id], nameToID[c])
			}
		} else {
			panic("configuration error")
		}
	}

	trace := network.GetTraceWithServices(traceFile, smap)

	flowMap := make(map[string]flowstats.Flow)
	for i := 0; i < len(trace.Trace); i++ {
		nextPkt := trace.Trace[i]
		if _, found := flowMap[nextPkt.FlowID]; !found {
			flow := *flowstats.CreateFlow()
			if sid, found := smap.GetId(nextPkt.Service); found {
				for _, counter := range serviceIdToCountersId[sid] {
					instance, _ := counters.InstantiateById(counter)
					flow.Cntrs = append(flow.Cntrs, instance)
				}
				flowMap[nextPkt.FlowID] = flow
			}

		}
	}
	N := int(trace.Count)
	allstart := time.Now()
	for i := 0; i < N; i++ {
		nextPkt := trace.Trace[i]
		f := flowMap[nextPkt.FlowID]
		f.AddPacket(&nextPkt.Pkt)
	}
	allend := time.Now()
	return float64(allend.Sub(allstart)) / float64(N)
}

func main() {
	trace := flag.String("trace", "", "Pcap trace to use for profiling. If none provide it runs on a default one")
	conf := flag.String("conf", "", "Configuration file to use for parsing traffic. If none provided it runs on a default one")
	folder := flag.String("folder", "", "Folder where to store the result. If none provided, it prints on stdout")
	flag.Parse()
	BenchmarkCPU(*trace, *conf, *folder)
}
