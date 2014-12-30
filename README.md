pomodogo
========

another golang app with a stupid punny name

Simple pomodoro-like timer that responds to two things:
- SIGUSR1, causing it to pause/resume the current session (if any)
- SIGUSR2, causing it to either start a new pomo session or stop any session currently running

Will pop up a prompt when a session ends; any input to the prompt causes the next session to begin.

Requires `dmenu` and intended for integration with i3 - for example:
```
; fgrep pomodogo ~/.i3/config 
bindsym $mod+p exec "pkill --signal SIGUSR2 pomodogo"
bindsym $mod+Shift+p exec "pkill --signal SIGUSR1 pomodogo"
```

TODO:
 - get rid of lock..
 - daemonize self, listen on socket, and invoke from command-line to signal instead of using signals
 - integrate with some non-terrible notification system
 - integrate with slock or similar
