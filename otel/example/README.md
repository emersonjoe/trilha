# Trilha + OpenTelemetry

A Trilha app of one page and one API route that exports a span per request to an
OTLP/HTTP collector. It is the smallest thing that shows a request crossing two services in
one trace.

## Run it

Start a collector that speaks OTLP/HTTP on `:4318` — the Grafana LGTM one-liner is the
shortest path to a trace you can look at:

```sh
docker run -p 3000:3000 -p 4318:4318 grafana/otel-lgtm
```

Then, from this directory:

```sh
OTEL_EXPORTER_OTLP_ENDPOINT=http://localhost:4318 \
OTEL_SERVICE_NAME=trilha-otel-example \
PORT=3001 go run .
```

Open <http://localhost:3001> (the collector UI above already takes 3000), then look for the
service in the collector's UI. Each request is one span named after the **route template** —
`/` and `/traco`, never the concrete path — carrying `http.request.method`,
`http.route`, `url.scheme`, `http.response.status_code`, `trilha.request_id` and, when the
session has one, `trilha.tenant`.

To see two services in one trace, point `EXAMPLE_TARGET_URL` at another instrumented service
and call `/traco`: the outbound call goes through `otel.Transport`, which puts this span's
`traceparent` on the wire.

Environment: `OTEL_EXPORTER_OTLP_ENDPOINT`, `OTEL_SERVICE_NAME`, `OTEL_SERVICE_VERSION`,
`OTEL_TRACES_SAMPLER_ARG` (the fraction of traces kept, `1` by default here),
`EXAMPLE_TARGET_URL`.

## Why it is not in `examples/`

Everything under `examples/` shares the root Go module, and the root module depends on the
standard library alone (constitution principle II, guarded by `TestNoExternalDeps`). Putting
this app there would drag the OpenTelemetry SDK into the framework's own module. It lives
inside the `otel/` module instead, which is the same isolation `bench/` uses, and is built
and tested by `make test-otel`.

## Em português

Um app Trilha mínimo que exporta um span por requisição para um coletor OTLP/HTTP. Suba um
coletor em `:4318`, rode `go run .` com `OTEL_EXPORTER_OTLP_ENDPOINT` apontando para ele e
veja cada requisição virar um span nomeado pelo **gabarito da rota** (`/blog/{slug}`, nunca
`/blog/meu-post`), com método, status, `request_id` e o id do tenant — nunca o caminho
concreto, a query string ou dado pessoal. A chamada de `/traco` sai por `otel.Transport`, que
põe o `traceparent` deste span na requisição: é assim que dois serviços aparecem no mesmo
trace. Este exemplo não mora em `examples/` porque aquela pasta compartilha o módulo da raiz,
que só depende da biblioteca padrão; ele vive dentro do módulo `otel/`, como o `bench/`.
