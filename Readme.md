# GoGPT

This is a high level helper library for OpenAI API. It is designed to be easy to use and to provide a simple interface for interacting with the OpenAI API.

I use this in several projects where I need to batch request asynchronous.

## usage

#### Scheduling batched
```golang
func main() {
    token := os.Getenv("OPENAI_API_KEY")
	g, err := gpt.NewGpt(token)
    if err != nil {
        log.Fatal(err)
    }

    var toTranslate []string = loadSnippetsToTranslate()
    
    applicationName := "my-translator"
    systemPrompt := `You are a translater translating snippets of text for an ecommerce shop to polish. Answer with a JSON object with the key "translation" and the value being the translated text.`

    currentSnippets := make([]string, 0)
    batches := make(map[string][]string)
    flush := func() {
        batchId, err := session.CreateBatch()
        if err != nil {
            log.Fatal(err)
        }
        batches[batchId] = currentSnippets
        currentSnippets = make([]string, 0)
        session := g.NewBatchSession()
    }

    session := g.NewBatchSession()
    for _, snippet := range toTranslate {
        reqId := fmt.Sprintf("%s-%d", applicationName, time.Now().UnixNano())
        err := session.AddToBatch(reqId, systemPrompt, snippet)
        if errors.Is(err, gpt.ErrExceedsFileLimit) {
            flush()
        }
        if err != nil {
            log.Fatal(err)
        }
        currentSnippets = append(currentSnippets, snippet)
    }
    if len(currentSnippets) > 0 {
        flush()
    }

    // somewhere store the snippets associated with batchId and lineIdx (idx of snippet in the batch)
}
```

#### Retrieve a batch
```golang

func main() {
    token := os.Getenv("OPENAI_API_KEY")
	g, err := gpt.NewGpt(token)
    if err != nil {
        log.Fatal(err)
    }

    var scheduledSnippets map[string][]string = loadScheduledSnippets()

    for batchId, snippets := range scheduledSnippets {
        for idx, snippet := range snippets {
    		rawResponse, err := session.RetrieveBatchedRequest(ctx, batchId, idx)
            if errors.Is(err, gpt.ErrBatchNotCompleted) {
			    continue
		    }
            if err != nil {
                log.Fatal(err)
            }
            
            var response struct {
			    Translation []string `json:"translation"`
		    }
            err = json.Unmarshal(rawResponse, &response)
            if err != nil {
                log.Fatal(err)
            }
            fmt.Printf("Snippet: %s -> %s\n", snippet, response.Translation)
        }
    }
}

```
