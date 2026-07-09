# Ruby examples

Pure-Ruby examples for the `kramdown` library as provided by
[go-embedded-ruby](https://github.com/go-embedded-ruby/ruby) (rbgo). Run them
with the `rbgo` interpreter:

```sh
rbgo examples/kramdown_usage.rb
```

| File | Shows |
| --- | --- |
| [`kramdown_usage.rb`](kramdown_usage.rb) | `Kramdown.to_html`, `Kramdown::Document`, an options Hash and `String#to_kramdown_html`. |

Each example is executed as-is under rbgo (`require "kramdown"`).
