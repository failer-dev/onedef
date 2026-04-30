# Why onedef

## 1. The LLM Era

We, software engineers, are living through an unprecedented shift. Coding ability is being commoditized at an alarming rate. It feels like watching the knight class collapse after gunpowder arrived.

Sure, people will say we've been through this before — punch cards → assembly → C → GC languages → frameworks. Automation kept coming, yet demand for developers only grew. But that's because each wave still required human hands. Better tools for the same craft.

LLM is different. The gun has arrived. And within years of the musket, we got the AK-47. We've entered an era that demands adaptation.

This isn't just doom. The opportunity is real, and the reward will be significant. The knight class fell — but those who adapted, who mastered leadership and new strategy, came out with more power and wealth than before.

## 2. The Bottleneck Has Shifted

LLM already builds end-to-end. Router registration, parameter parsing, test coverage — all of it. The bottleneck of raw implementation is approaching zero.

But the bottleneck has moved. It's now **verification**.

LLM output is non-deterministic. Humans have to review it. Sure, most people just commit and push without looking — but that's because reviewing is genuinely hard, and nobody can honestly deny it. Checking a single API means opening multiple files, and you need to deeply know the language to even know what you're looking at.

So verification gets skipped. And when verification gets skipped, LLM drifts somewhere no one controls.

## 3. The Solution

```go
type GetUserAPI struct {
    onedef.GET `path:"/users/{id}"`
    Request    struct{ ID string }
    Response   User
}
```

A developer who doesn't know Go can read this struct in five seconds. It's a GET. The path is `/users/{id}`. It takes an ID and returns a User. No need to open the router file. No need to check if the SDK was updated. Change this struct, and everything else follows structurally.

The range of people who can verify expands. A developer unfamiliar with the backend can review a PR. A Flutter developer can direct LLM to build a read API themselves.

Don't use LLM as a tool.

Make your project a tool for LLM.

