| Signal | Number | Default Behavior | Can be caught? | 
| --- | --- | --- | --- | 
| SIGTERM | 15 | Graceful shutdown request | Yes
| SIGKILL | 9 | Immediate kill | No |
| SIGHUP | 1 | Hangup / reload config | Yes |
| SIGINT | 2 | Interrupt (Ctrl+C) | Yes |
| SIGCHLD | 17 | Child process changed state | Yes |
| SIGSTOP | 19 | Pause process | No |
| SIGCONT | 18 | Resume paused process | Yes | 