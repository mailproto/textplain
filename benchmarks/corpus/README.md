# Benchmark corpus

`tiny`, `article`, `receipt` and `pathological` are generated for this benchmark.

`mjml-newsletter.html` stands in for the output of an email builder: nested layout tables,
conditional comments for Outlook, a hidden preview and inline styles on nearly every element.
It is compiled from `mjml-newsletter.mjml`, written for this corpus, with
[MJML](https://github.com/mjmlio/mjml) 4.18.0: `npx mjml@4.18.0 mjml-newsletter.mjml -o mjml-newsletter.html`.

The rest are real-world email templates, both MIT licensed, included unmodified:

- `cerberus-fluid.html`, `cerberus-responsive.html` — https://github.com/TedGoas/Cerberus
- `leemunroe.html` — https://github.com/leemunroe/responsive-html-email-template
