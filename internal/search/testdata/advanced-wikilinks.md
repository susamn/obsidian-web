---
title: Advanced Wikilinks Test
tags:
  - test
  - wikilinks
---

# Advanced Wikilinks Test

This file tests advanced wikilink formats with block references and aliases.

## Block References with Aliases

Here's a link with block ref and alias: [[Date Formats#^e4a164|RFC3339]]

This should index "Date Formats" only, stripping the block ref (#^e4a164) and alias (|RFC3339).

## Multiple Block References to Same Note

First reference: [[Golang FAQs#^46e652]]
Second reference: [[Golang FAQs#^ab5315|FAQ Link]]

Both should deduplicate to just "Golang FAQs".

## Embeds with Block References

Embedding content: ![[Golang FAQs#^46e652]]
Another embed: ![[Date Formats#^abc123|formatted date]]

## Regular Wikilinks (No Block Refs)

Simple link: [[Getting Started]]
Link with heading: [[Installation#Prerequisites]]
Link with alias: [[Configuration|Config]]

## Mixed Format

- [[random]]
- [[HTTP]]
- [[Golang]]
- [[JSON#Examples|json examples]]
- [[POST#^blockid|post method]]
