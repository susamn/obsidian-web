# Mixed Wikilinks Test

This note has various wikilink formats mixed together.

Basic: [[Note1]], [[Note2]]
With aliases: [[Note3|alias3]], [[Note4|See this]]
With headings: [[Note5#Section1]], [[Note6#Introduction]]
Combined: [[Note7#Section|custom alias]], [[Note8#Heading|link text]]

Duplicates should be deduplicated:
- [[Note1]]
- [[Note1]]
- [[Note3|different alias]]
- [[Note3|another alias]]
