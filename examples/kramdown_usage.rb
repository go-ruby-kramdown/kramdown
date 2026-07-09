# frozen_string_literal: true
#
# Pure-Ruby usage of the Kramdown module, as provided by go-embedded-ruby (rbgo).
# Run it with:  rbgo examples/kramdown_usage.rb

require "kramdown"

# One-shot convenience: render a Markdown source String to an HTML String.
# Headings get an auto-generated id by default (:auto_ids is on).
puts Kramdown.to_html("# Title\n\nHello **world**.")
# => <h1 id="title">Title</h1>\n\n<p>Hello <strong>world</strong>.</p>

# Kramdown::Document is the gem's public entry point: it wraps the source and
# renders on demand through #to_html.
doc = Kramdown::Document.new("A paragraph with *emphasis*, `code` and [a link](https://example.com).")
puts doc.to_html

# Lists, blockquotes and indented code blocks all render to their HTML shapes.
puts Kramdown.to_html("- one\n- two\n- three")

# Options are passed as a Hash; here we disable the automatic heading ids.
puts Kramdown.to_html("## Section", { auto_ids: false })
# => <h2>Section</h2>

# String#to_kramdown_html is an rbgo convenience for rendering a String directly.
puts "> a quoted line".to_kramdown_html
