package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"strconv"
	"strings"
)

const (
	mockJSONName = "qstat.json"
)

type Job struct {
	ID    string            `json:"id"`
	Attrs map[string]string `json:"attributes"`
}

const (
	attrJobOwner              = "Job_OWner"
	attrJobName               = "Job_Name"
	attrJobArrayID            = "job_array_id"
	attrExecHost              = "exec_host"
	attrJobState              = "job_state"
	attrResourcesUsedMem      = "resources_used.mem"
	attrResourcesUsedCPUTime  = "resources_used.cput"
	attrResourcesUsedWalltime = "resources_used.walltime"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}

func run() error {
	file, err := os.Open(mockJSONName)
	if err != nil {
		return err
	}
	defer file.Close()

	data, err := ioutil.ReadAll(file)
	if err != nil {
		return err
	}

	var jobs []Job
	if err := json.Unmarshal(data, &jobs); err != nil {
		return err
	}

	usage := map[string]map[string]int{}

	for _, job := range jobs {
		for host, coreCount := range job.occupancy() {
			if _, ok := usage[host]; !ok {
				usage[host] = map[string]int{}
			}
			usage[host][job.baseName()] += coreCount
		}
	}

	for host, jobs := range usage {
		load := 0
		for _, cores := range jobs {
			load += cores
		}

		fmt.Printf(
			"\x1b[36m%s\x1b[m \x1b[90m[%d]\x1b[m \x1b[32m%s\x1b[m\n",
			host,
			load,
			strings.Repeat("|", load),
		)
	}

	return nil
}

func (job *Job) baseName() string {
	name := job.Attrs[attrJobName]
	index, ok := job.Attrs[attrJobArrayID]
	if ok {
		name = strings.TrimSuffix(name, "-"+index)
	}
	return name
}

func (job *Job) occupancy() map[string]int {
	occ := map[string]int{}

	execHost, ok := job.Attrs[attrExecHost]
	if !ok {
		return occ
	}

	// exec_host  = host_cores *("+" host_cores)
	// host_cores = host "/" cores
	// cores      = core_range *("," core_range)
	// core_range = core | core "-" core

	hostCoresArray := strings.Split(execHost, "+")
	for _, hostCores := range hostCoresArray {
		host, cores, _ := bisect(hostCores, "/")
		coreRanges := strings.Split(cores, ",")
		for _, coreRange := range coreRanges {
			if a, b, ok := bisect(coreRange, "-"); ok {
				first, _ := strconv.Atoi(a)
				last, _ := strconv.Atoi(b)
				occ[host] += last - first + 1
			} else {
				occ[host]++
			}
		}
	}

	return occ
}

func bisect(s string, delim string) (string, string, bool) {
	pos := strings.Index(s, delim)
	if pos == -1 {
		return s, "", false
	}
	return s[:pos], s[pos+1:], true
}
