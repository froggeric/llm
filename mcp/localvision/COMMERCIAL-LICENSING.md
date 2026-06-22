# Commercial Licensing

## Summary

`localvision` is distributed under the
[PolyForm Noncommercial License 1.0.0](https://polyformproject.org/licenses/noncommercial/1.0.0).
That license is *source-available*: you can read, modify, and redistribute the
code, but you may **not** use it for commercial purposes without a separate
commercial license from the author.

## What counts as "commercial"?

The PolyForm Noncommercial license defines its purpose as:

> Licensor grants you the right to use the software for any non-commercial
> purpose.

In plain language, the following are **not commercial** and need no extra
license:

- Personal projects, hobby use, learning, education, academic research.
- Internal use inside a company **solely** to support that company's own
  engineering work, provided the MCP itself is not being resold, hosted as
  a paid service, or bundled into a product you ship to customers.
- Contributing fixes back to this repository.

The following **are** commercial and require a separate license:

- Bundling `localvision` (or a derivative) into a product you sell,
  license, or offer as a paid SaaS.
- Offering `localvision` as a hosted, metered, or paid service.
- Embedding it in a paid agent framework, IDE, or developer tool.
- Internal use at scale where the MCP is a material component of a service
  you charge customers for.

The full text of the license is authoritative; the summary above is just a
pointer. Read the actual license at
<https://polyformproject.org/licenses/noncommercial/1.0.0>.

## Bundled components

`localvision` downloads and runs `llama-server` from the upstream
[`llama.cpp`](https://github.com/ggml-org/llama.cpp) project at runtime.
`llama.cpp` is MIT-licensed. A commercial license for `localvision`
covers only the wrapper code in this repository; it does **not** change the
license of the upstream `llama.cpp` binary, which remains MIT.

Model weights are downloaded from `huggingface.co/froggeric/` and are subject
to their own model licenses (typically Apache-2.0 or MIT for the models
shipped in v0.1). A commercial license for `localvision` does not
override any model-specific license terms.

## Requesting a commercial license

For commercial licensing, paid support, or to discuss a custom arrangement
(SaaS, embedded use, enterprise rollouts, indemnification), contact:

**Frederic Guigand — <frederic@guigand.com>**

Please include:

1. The legal name of the entity that will hold the license.
2. How you intend to use `localvision` (product name, deployment model,
   rough scale).
3. Whether you need support, indemnification, or a custom SLA.

You will typically get an initial response within two business days. Simple
"we want to use this internally at a paid product company" licenses are
usually quick; SaaS / redistribution arrangements take longer.

## Why PolyForm Noncommercial?

Two reasons:

1. **The author wants this tool to stay free for individual developers and
   open research**, while preventing a third party from simply re-hosting it
   as a paid service.
2. It keeps the door open to dual-licensing (a future Apache-2.0 release, an
   enterprise tier) without retroactively stripping the noncommercial
   promise from people who already adopted it.

If you have a strong opinion that this should be a permissive license
instead, the contact address above is the right place to make that case.
