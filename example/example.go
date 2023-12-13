package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/nbd-wtf/go-nostr"
	"github.com/nbd-wtf/go-nostr/nip19"
)

func main() {
	ctx, cancel := context.WithTimeout(context.Background(), 3000*time.Second)
	// ctx := context.Background()

	// connect to relay
	url := "wss://relay.nostrassets.com"
	relay, err := nostr.RelayConnect(ctx, url)
	if err != nil {
		panic(err)
	}

	fmt.Printf("relay: %+v\n", relay.IsConnected())

	reader := os.Stdin
	var npub string
	var b [64]byte
	fmt.Fprintf(os.Stderr, "using %s\n----\nexample subscription for three most recent notes mentioning user\npaste npub key: ", url)
	if n, err := reader.Read(b[:]); err == nil {
		npub = strings.TrimSpace(fmt.Sprintf("%s", b[:n]))
	} else {
		panic(err)
	}

	// create filters
	var filters nostr.Filters
	if _, v, err := nip19.Decode(npub); err == nil {
		t := make(map[string][]string)
		// making a "p" tag for the above public key.
		// this filters for messages tagged with the user, mainly replies.
		t["p"] = []string{v.(string)}
		filters = []nostr.Filter{{
			Kinds: []int{nostr.KindEncryptedDirectMessage, nostr.KindTextNote},
			Tags:  t,
			// limit = 3, get the three most recent notes
			Limit: 3,
		}}
	} else {
		panic("not a valid npub!")
	}

	fmt.Println("filters: ", filters)

	// create a subscription and submit to relay
	// results will be returned on the sub.Events channel
	sub, err := relay.Subscribe(ctx, filters)
	if err != nil {
		fmt.Println("subscribe err: ", err)
		return
	}

	// we will append the returned events to this slice
	evs := make([]nostr.Event, 0)

	go func() {
		<-sub.EndOfStoredEvents
		cancel()
		fmt.Println("end event")
	}()
	for ev := range sub.Events {
		evs = append(evs, *ev)
	}

	filename := "example_output.json"
	if f, err := os.Create(filename); err == nil {
		fmt.Fprintf(os.Stderr, "returned events saved to %s\n", filename)
		// encode the returned events in a file
		enc := json.NewEncoder(f)
		enc.SetIndent("", " ")
		enc.Encode(evs)
		f.Close()
	} else {
		panic(err)
	}

	fmt.Fprintf(os.Stderr, "----\nexample publication of note.\npaste nsec key (leave empty to autogenerate): ")
	var nsec string
	if n, err := reader.Read(b[:]); err == nil {
		nsec = strings.TrimSpace(fmt.Sprintf("%s", b[:n]))
	} else {
		panic(err)
	}

	var sk string
	ev := nostr.Event{}
	if _, s, e := nip19.Decode(nsec); e == nil {
		sk = s.(string)
	} else {
		sk = nostr.GeneratePrivateKey()
	}
	if pub, e := nostr.GetPublicKey(sk); e == nil {
		ev.PubKey = pub
		if npub, e := nip19.EncodePublicKey(pub); e == nil {
			fmt.Fprintln(os.Stderr, "using:", npub)
		}
	} else {
		panic(e)
	}

	ev.CreatedAt = nostr.Now()
	ev.Kind = nostr.KindEncryptedDirectMessage
	var content string
	fmt.Fprintln(os.Stderr, "enter content of note, ending with an empty newline (ctrl+d):")
	for {
		if n, err := reader.Read(b[:]); err == nil {
			content = fmt.Sprintf("%s%s", content, fmt.Sprintf("%s", b[:n]))
		} else if err == io.EOF {
			break
		} else {
			panic(err)
		}
	}
	ev.Content = strings.TrimSpace(content)
	ev.Sign(sk)

	for _, url := range []string{"wss://relay.nostrassets.com", "wss://relay.primal.net", "wss://relay.damus.io", "wss://nos.lol"} {
		ctx := context.WithValue(context.Background(), "url", url)
		relay, e := nostr.RelayConnect(ctx, url)
		if e != nil {
			fmt.Println(e)
			continue
		}

		fmt.Println("posting to: ", url)
		err = relay.Publish(ctx, ev)
		if err != nil {
			fmt.Println("Publish err", err.Error())
			continue
		}

		evts, err := relay.QuerySync(ctx, filters[len(filters)-1], nil)
		if err != nil {
			fmt.Println("QuerySync err", err.Error())
			continue
		}
		fmt.Println("listen to events: ", url, evts)
	}
}
