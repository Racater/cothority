// deploy2deter is responsible for kicking off the deployment process
// for deterlab. Given a list of hostnames, it will create an overlay
// tree topology, using all but the last node. It will create multiple
// nodes per server and run timestamping processes. The last node is
// reserved for the logging server, which is forwarded to localhost:8081
//
// options are "bf" which specifies the branching factor
//
// 	and "hpn" which specifies the replicaiton factor: hosts per node
//
// Creates the following directory structure in remote:
// exec, timeclient, logserver/...,
// this way it can rsync the remove to each of the destinations
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"sync"

	"github.com/ineiti/cothorities/helpers/cliutils"
	"github.com/ineiti/cothorities/helpers/config"
	"github.com/ineiti/cothorities/helpers/graphs"
	dbg "github.com/ineiti/cothorities/helpers/debug_lvl"
)

// bf is the branching factor of the tree that we want to build
var bf int

// hpn is the replication factor of hosts per node: how many hosts do we want per node
var hpn int

var nmsgs int
var debug int
var rate int
var failures int
var rFail int
var fFail int
var kill bool
var rounds int
var nmachs int
var testConnect bool
var app string
var suite string
var build bool
var user string
var host string
var nloggers int
var masterLogger string
var phys []string
var virt []string
var physOut string
var virtOut string

func init() {
	flag.IntVar(&bf, "bf", 2, "branching factor: default binary")
	flag.IntVar(&hpn, "hpn", 1, "hosts per node: default 1")
	flag.IntVar(&nmsgs, "nmsgs", 100, "number of messages per round")
	flag.IntVar(&rate, "rate", -1, "number of milliseconds between messages: if rate > 0 then used")
	flag.IntVar(&debug, "debug", 2, "debugging-level: 0 is silent, 5 is flooding")
	flag.IntVar(&failures, "failures", 0, "percent showing per node probability of failure")
	flag.IntVar(&rFail, "rfail", 0, "number of consecutive rounds each root runs before it fails")
	flag.IntVar(&fFail, "ffail", 0, "number of consecutive rounds each follower runs before it fails")
	flag.IntVar(&rounds, "rounds", 100, "number of rounds to run for")
	flag.BoolVar(&kill, "kill", false, "kill all running processes (but don't start anything)")
	flag.IntVar(&nmachs, "nmachs", 20, "number of machines to use")
	flag.IntVar(&nloggers, "nloggers", 3, "number of loggers in pool")
	flag.BoolVar(&testConnect, "test_connect", false, "test connecting and disconnecting")
	flag.StringVar(&app, "app", "sign", "app to run")
	flag.StringVar(&suite, "suite", "ed25519", "abstract suite to use [nist256, nist512, ed25519]")
	flag.BoolVar(&build, "build", false, "build all helpers with go")
	flag.StringVar(&user, "user", "ineiti", "User on the deterlab-machines")
	flag.StringVar(&host, "host", "users.deterlab.net", "Hostname of the deterlab")
}


func main() {
	flag.Parse()
	dbg.DebugVisible = debug
	dbg.Lvl2("RUNNING DEPLOY2DETER WITH RATE:", rate, " on machines:", nmachs)

	os.MkdirAll("remote", 0777)
	readHosts()

	// killssh processes on users
	dbg.Lvl2("Stopping programs on user.deterlab.net")
	cliutils.SshRunStdout(user, host, "killall ssh scp deter 2>/dev/null 1>/dev/null")

	// If we have to build, we do it for all programs and then copy them to 'host'
	if build {
		doBuild()
	}

	killssh := exec.Command("pkill", "-f", "ssh -t -t")
	killssh.Stdout = os.Stdout
	killssh.Stderr = os.Stderr
	err := killssh.Run()
	if err != nil {
		log.Print("Stopping ssh: ", err)
	}

	calculateGraph()

	// Copy everything over to deterlabs
	err = cliutils.Rsync(user, host, "remote", "")
	if err != nil {
		log.Fatal(err)
	}

	// setup port forwarding for viewing log server
	// ssh -L 8081:pcXXX:80 username@users.isi.deterlab.net
	// ssh username@users.deterlab.net -L 8118:somenode.experiment.YourClass.isi.deterlab.net:80
	fmt.Println("setup port forwarding for master logger: ", masterLogger)
	cmd := exec.Command(
		"ssh",
		"-t",
		"-t",
		fmt.Sprintf("%s@%s", user, host),
		"-L",
		"8081:" + masterLogger + ":10000")
	err = cmd.Start()
	if err != nil {
		log.Fatal("failed to setup portforwarding for logging server")
	}

	dbg.Lvl2("runnning deter with nmsgs:", nmsgs)
	// run the deter lab boss nodes process
	// it will be responsible for forwarding the files and running the individual
	// timestamping servers
	dbg.Lvl2(cliutils.SshRunStdout(user, host,
		"GOMAXPROCS=8 remote/deter -nmsgs=" + strconv.Itoa(nmsgs) +
		" -hpn=" + strconv.Itoa(hpn) +
		" -bf=" + strconv.Itoa(bf) +
		" -rate=" + strconv.Itoa(rate) +
		" -rounds=" + strconv.Itoa(rounds) +
		" -debug=" + strconv.Itoa(debug) +
		" -failures=" + strconv.Itoa(failures) +
		" -rfail=" + strconv.Itoa(rFail) +
		" -ffail=" + strconv.Itoa(fFail) +
		" -test_connect=" + strconv.FormatBool(testConnect) +
		" -app=" + app +
		" -suite=" + suite +
		" -kill=" + strconv.FormatBool(kill)))

	dbg.Lvl2("END OF DEPLOY2DETER")
}

func readHosts() {
	// parse the hosts.txt file to create a separate list (and file)
	// of physical nodes and virtual nodes. Such that each host on line i, in phys.txt
	// corresponds to each host on line i, in virt.txt.
	physVirt, err := cliutils.ReadLines("hosts.txt")
	if err != nil {
		log.Panic("Couldn't find hosts.txt")
	}

	phys = make([]string, 0, len(physVirt) / 2)
	virt = make([]string, 0, len(physVirt) / 2)
	for i := 0; i < len(physVirt); i += 2 {
		phys = append(phys, physVirt[i])
		virt = append(virt, physVirt[i + 1])
	}
	// only use the number of machines that we need
	if nmachs + nloggers > len(phys) {
		log.Fatal("Error, having only ", len(phys), " hosts while ", nmachs + nloggers, " are needed")
	}
	phys = phys[:nmachs + nloggers]
	virt = virt[:nmachs + nloggers]
	physOut = strings.Join(phys, "\n")
	virtOut = strings.Join(virt, "\n")
	masterLogger = phys[0]
	// slaveLogger1 := phys[1]
	// slaveLogger2 := phys[2]

	// phys.txt and virt.txt only contain the number of machines that we need
	dbg.Lvl2("Reading phys and virt")
	err = ioutil.WriteFile("remote/phys.txt", []byte(physOut), 0666)
	if err != nil {
		log.Fatal("failed to write physical nodes file", err)
	}

	err = ioutil.WriteFile("remote/virt.txt", []byte(virtOut), 0666)
	if err != nil {
		log.Fatal("failed to write virtual nodes file", err)
	}

	err = exec.Command("cp", "remote/phys.txt", "remote/virt.txt", "remote/logserver/").Run()
	if err != nil {
		log.Fatal("error copying phys, virt, and remote/logserver:", err)
	}
}

func calculateGraph() {
	virt = virt[3:]
	phys = phys[3:]
	t, hostnames, depth, err := graphs.TreeFromList(virt, hpn, bf)
	dbg.Lvl2("DEPTH:", depth)
	dbg.Lvl2("TOTAL HOSTS:", len(hostnames))

	dbg.Lvl2("Going to generate tree-lists")
	b, err := json.Marshal(t)
	if err != nil {
		log.Fatal("unable to generate tree from list")
	}
	err = ioutil.WriteFile("remote/logserver/cfg.json", b, 0660)
	if err != nil {
		log.Fatal("unable to write configuration file")
	}

	// NOTE: now remote/logserver is ready for transfer
	// it has logserver/ folder, binary, and cfg.json, and phys.txt, virt.txt

	// generate the configuration file from the tree
	cf := config.ConfigFromTree(t, hostnames)
	cfb, err := json.Marshal(cf)
	err = ioutil.WriteFile("remote/cfg.json", cfb, 0666)
	if err != nil {
		log.Fatal(err)
	}
}

func doBuild() {

	var wg sync.WaitGroup

	// start building the necessary packages
	dbg.Lvl2("Starting to build all executables")
	packages := []string{"../logserver", "../timeclient", "../forkexec", "../exec", "../deter"}
	//packages := []string{"../deter"}
	//packages := []string{"../logserver"}
	for _, p := range packages {

		dbg.Lvl2("Building ", p)
		wg.Add(1)
		if p == "../deter" {
			go func(p string) {
				defer wg.Done()
				// the users node has a 386 FreeBSD architecture
				err := cliutils.Build(p, "386", "freebsd")
				if err != nil {
					cliutils.KillGo()
					log.Fatal(err)
				}
			}(p)
			continue
		}
		go func(p string) {
			defer wg.Done()
			// deter has an amd64, linux architecture
			err := cliutils.Build(p, "amd64", "linux")
			if err != nil {
				cliutils.KillGo()
				log.Fatal(err)
			}
		}(p)
	}
	// wait for the build to finish
	wg.Wait()
	dbg.Lvl2("Build is finished")

	// copy the logserver directory to the current directory
	err := exec.Command("cp", "-a", "../logserver", "remote/").Run()
	if err != nil {
		log.Fatal("error copying logserver directory into remote directory:", err)
	}

	err = exec.Command("cp", "-a", "logserver", "remote/logserver/logserver").Run()
	if err != nil {
		log.Fatal("error renaming logserver:", err)
	}

	// scp the files that we need over to the boss node
	cmd := exec.Command("cp", "timeclient", "exec", "forkexec", "deter", "remote/")
	err = cmd.Run()
	if err != nil {
		log.Fatal("error unable to copy files into remote directory:", err)
	}
	dbg.Lvl2("Done building")
}
