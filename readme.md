# aicmd

usage: 

```
$ aicmd "find all video files"
generated command: find /path/to/search -type f -iname "*.mp4" -o -iname "*.mkv" -o -iname "*.avi" -o -iname "*.mov" -o -iname "*.flv" -o -iname "*.wmv"

run it now? [Y/n]:
```

aicmd calls openai and asks it to generate a bash command for you, then optionally runs it