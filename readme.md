> [!TIP]
> After using this for a few months I developed some opinions on how it should work that are better served by my new [ai-completion](https://github.com/XanderStrike/ai-completion) bash/zsh function. Consider using that instead.

# aicmd

use AI to generate swanky bash one-liners (supports Anthropic, OpenAI, and Ollama!)

## usage

```
$ aicmd "find duplicate files in the current directory recursively, show their sizes, and group them by content"
generated command: find . -type f -exec sha256sum {} \; | sort | awk '{ if (last == $1) { if (count == 1) print prev; print $0; count++ } else { last = $1; prev = $0; count = 1 } }' | while read -r hash file; do size=$(du -h "$file" | cut -f1); echo -e "$size\t$file"; done | sort -h | awk '{ if (NR==1 || prev != $1) print "\nSize: " $1; prev=$1; print "  → " $2 }'

run it now? [Y/n]: y

Size: 4.2M
  → ./photos/vacation2023/IMG_1234.jpg
  → ./backup/old_photos/IMG_1234.jpg

Size: 8.7M
  → ./downloads/project-v1.0.0.tar.gz
  → ./archives/project-v1.0.0.tar.gz
  → ./desktop/project-backup.tar.gz

Size: 15M
  → ./videos/screencast.mp4
  → ./uploads/demo_video.mp4
```

## installing

Pre-built binaries are available for Linux (AMD64/ARM64) and macOS (ARM64) on the [releases page](https://github.com/XanderStrike/aicmd/releases). Download them and put them in your path or see below to build them.

Set API keys must be set as environment variables, if multiple are set `aicmd` prefers anthropic, then openai, then ollama. You can also pass `--provider <anthropic|openai|ollama>` and `--model <whatever>`.


```bash
export ANTHROPIC_API_KEY="your-api-key"
aicmd "find duplicate files"
```

```bash
export OPENAI_API_KEY="your-api-key"
aicmd "find duplicate files"
```

```bash
export OLLAMA_API_BASE="http://localhost:11434"
export OLLAMA_MODEL="mistral"  # or any other model you've pulled
aicmd "find duplicate files"
```

## building

1. **Install Go**: Make sure Go is installed. [Download it here](https://go.dev/dl/).

2. **Get the Code**: Clone the repo and navigate into it.

   ```bash
   git clone <repo-url>
   cd <repo-directory>
   ```

3. **Set API Key**: Choose your AI provider:

   For Anthropic (default):
   ```bash
   export ANTHROPIC_API_KEY="your-api-key"
   ```

   For OpenAI:
   ```bash
   export OPENAI_API_KEY="your-api-key"
   ```

   For Ollama:
   ```bash
   export OLLAMA_API_BASE="http://localhost:11434"
   export OLLAMA_MODEL="mistral"  # or whatever model you prefer
   ```

   You can also specify the provider explicitly with the `--provider` flag:
   ```bash
   aicmd --provider openai "your command"
   aicmd --provider anthropic "your command"
   aicmd --provider ollama "your command"
   ```

4. **Build**: Run the build command.

   ```bash
   go build -o aicmd main.go
   ```

5. **Run**: Use the tool.

   ```bash
   ./aicmd "describe your task"
   ```

6. **Optional**: Move `aicmd` to a directory in your `PATH` for easy access.

   ```bash
   sudo mv aicmd /usr/local/bin/
   ```
