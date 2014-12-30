package main

import (
	"bytes"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"sync"
	"syscall"
	"time"
)

const (
	modeWork = iota
	modeRest
	modeIdle
)

var (
	prevMode = modeRest // what mode were we just in?
	mode     = modeIdle // what mode are we in now?

	workTime time.Duration
	restTime time.Duration

	prCh = make(chan struct{}) // global pause/resume trigger
	ssCh = make(chan struct{}) // global stop/start trigger

	slock = sync.Mutex{} // prevent signals from stepping on each other
)

// prompt invokes dmenu to pop up a small prompt acknowledging the end of a
// session and requiring user intervention to trigger the next one.
func prompt(from, to string) {
	args := []string{
		"dmenu",
		"-nb", "#151515",
		"-nf", "#999999",
		"-sb", "#f00060",
		"-sf", "#000000",
		"-fn", "-*-*-medium-r-normal-*-*-*-*-*-*-100-*-*",
		//		"-fn", "pango:DejaVu Sans Mono 14",
		"-i",
		"-p", fmt.Sprintf(`'%s ended. %s time!'`, from, to),
	}
	c := exec.Command(args[0], args[1:]...)
	// TODO(jonboulle): what makes sense here?
	c.Stdin = bytes.NewBufferString("OK")
	//	c.Stdout = os.Stdout
	//	c.Stderr = os.Stderr
	fmt.Println("Prompting user")
	//	fmt.Printf("Running %q\n", strings.Join(c.Args, " "))
	// TODO(jonboulle): handle error or what?
	c.Run()
}

// handleStopStart acts in response to SIGUSR1 and either stops a currently
// running work or rest session, or starts a new work session
func handleStopStart(ch <-chan os.Signal) {
	doneCh := make(chan struct{})
	for {
		select {
		case <-doneCh:
			slock.Lock()
			doneCh = make(chan struct{})
			switch mode {
			case modeRest:
				prevMode = mode
				mode = modeWork
				prompt("Rest", "Pomodoro")
				go run("work", workTime, doneCh)
			case modeWork:
				prevMode = mode
				mode = modeRest
				prompt("Pomodoro", "Rest")
				go run("rest", restTime, doneCh)
			default:
				panic("bad mode!")
			}
			slock.Unlock()
		case s := <-ch:
			slock.Lock()
			fmt.Printf("Received %v - ", s)
			doneCh = make(chan struct{})
			switch mode {
			case modeWork, modeRest:
				fmt.Println("stopping", mode)
				close(ssCh)
				// when stopping, next mode is always work
				prevMode = modeRest
				mode = modeIdle
			case modeIdle:
				switch prevMode {
				case modeRest:
					ssCh = make(chan struct{})
					mode = modeWork
					go run("work", workTime, doneCh)
				default:
					panic("unexpected prevMode!")
				}
			default:
				panic("bad mode!")
			}
			slock.Unlock()
		}
	}
}

// handlePauseResume acts in response to SIGUSR2 and pauses or resumes any
// current work or rest session
func handlePauseResume(ch <-chan os.Signal) {
	for {
		s := <-ch
		slock.Lock()
		fmt.Printf("Received %v - ", s)
		switch mode {
		case modeWork, modeRest:
			fmt.Println("triggering pause/resume")
			prCh <- struct{}{}
		case modeIdle:
			fmt.Println("idle, ignoring")
			// Do nothing
		}
		slock.Unlock()
	}
}

func main() {
	flag.DurationVar(&workTime, "ptime", 25*time.Minute, "length of each work session")
	flag.DurationVar(&restTime, "rtime", 5*time.Minute, "length of each rest session")
	flag.Parse()

	sigPrCh := make(chan os.Signal, 1)
	sigSsCh := make(chan os.Signal, 1)
	go handleStopStart(sigSsCh)
	go handlePauseResume(sigPrCh)

	signal.Notify(sigPrCh, syscall.SIGUSR2)
	signal.Notify(sigSsCh, syscall.SIGUSR1)

	// Loop forever - we're only controlled via signals
	fmt.Printf("pomodogo started with pid %v. Sleeping...\n", os.Getpid())
	<-make(chan struct{})
}

// run counts down from the given duration.
// If the countdown expires, done will be closed.
// If the global pause/resume channel is triggered, the countdown is paused/resumed.
// If the global stop/start channel is triggered, the countdown is stopped.
func run(typ string, t time.Duration, done chan<- struct{}) {
	fmt.Printf("starting new %s session\n", typ)
	countdown := t.Seconds()
	ticker := time.Tick(time.Second)
	paused := false
	for {
		if countdown <= 0 {
			close(done)
			fmt.Printf("%s session done\n", typ)
			return
		}
		select {
		case <-ticker:
			if !paused {
				fmt.Printf("- %s session tick (%v)\n", typ, countdown)
				countdown -= 1
			}
		case <-prCh:
			paused = !paused
			if paused {
				fmt.Printf("- %s session paused\n", typ)
			} else {
				fmt.Printf("- %s session unpaused\n", typ)
			}
		case <-ssCh:
			fmt.Printf("%s session stopped\n", typ)
			return
		}
	}
}

// notifications would be nice but I cannot seem to find a daemon that properly
// supports the timeouts that libnotify clients send, ugh
//	import notify "github.com/mqu/go-notify"
/*
	notify.Init("pomodogo")
	hi := notify.NotificationNew("hi world", "this is an ex", "")
	if hi == nil {
		panic("no notification")
	}
	notify.NotificationSetTimeout(hi, 5)
	if err := notify.NotificationShow(hi); err != nil {
		fmt.Println(err.Message())
		panic(err)
	}
	return
*/
