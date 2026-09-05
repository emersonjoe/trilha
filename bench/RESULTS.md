# Resultados de referência

Gerado em 2026-09-05 por `make bench-results` — go1.25.1, Apple M2, Darwin arm64.
Custo por requisição em processo (`httptest`), só o framework; leia a metodologia em
https://emersonjoe.github.io/trilha/referencia/desempenho.

```
BenchmarkHealthLive-8           	 1000000	      1015 ns/op	    1252 B/op	      18 allocs/op
BenchmarkHealthLive-8           	 1294849	       973.7 ns/op	    1252 B/op	      18 allocs/op
BenchmarkHealthLive-8           	 1295538	       883.1 ns/op	    1252 B/op	      18 allocs/op
BenchmarkJSON_Stdlib-8          	  276159	      4193 ns/op	    4428 B/op	      10 allocs/op
BenchmarkJSON_Stdlib-8          	  288009	      4316 ns/op	    4428 B/op	      10 allocs/op
BenchmarkJSON_Stdlib-8          	  288871	      4214 ns/op	    4427 B/op	      10 allocs/op
BenchmarkJSON_Trilha-8          	  141925	      8370 ns/op	    6577 B/op	      51 allocs/op
BenchmarkJSON_Trilha-8          	  146773	      8025 ns/op	    6577 B/op	      51 allocs/op
BenchmarkJSON_Trilha-8          	  156602	      7753 ns/op	    6577 B/op	      51 allocs/op
BenchmarkMiddleware5_Stdlib-8   	 1824530	       672.8 ns/op	    1016 B/op	      10 allocs/op
BenchmarkMiddleware5_Stdlib-8   	 1836939	       658.8 ns/op	    1016 B/op	      10 allocs/op
BenchmarkMiddleware5_Stdlib-8   	 1840015	       666.5 ns/op	    1016 B/op	      10 allocs/op
BenchmarkMiddleware5_Trilha-8   	  258580	      4087 ns/op	    3440 B/op	      51 allocs/op
BenchmarkMiddleware5_Trilha-8   	  293600	      4579 ns/op	    3440 B/op	      51 allocs/op
BenchmarkMiddleware5_Trilha-8   	  295340	      4136 ns/op	    3440 B/op	      51 allocs/op
BenchmarkPage_Stdlib-8          	   41977	     31232 ns/op	   18944 B/op	     270 allocs/op
BenchmarkPage_Stdlib-8          	   42008	     28584 ns/op	   18944 B/op	     270 allocs/op
BenchmarkPage_Stdlib-8          	   42255	     28632 ns/op	   18944 B/op	     270 allocs/op
BenchmarkPage_Trilha-8          	   52748	     20858 ns/op	   28242 B/op	     482 allocs/op
BenchmarkPage_Trilha-8          	   61194	     19862 ns/op	   28241 B/op	     482 allocs/op
BenchmarkPage_Trilha-8          	   61419	     19693 ns/op	   28241 B/op	     482 allocs/op
BenchmarkPing_MetricsOff-8      	  304446	      4057 ns/op	    3152 B/op	      50 allocs/op
BenchmarkPing_MetricsOff-8      	  312129	      4058 ns/op	    3152 B/op	      50 allocs/op
BenchmarkPing_MetricsOff-8      	  320017	      4508 ns/op	    3152 B/op	      50 allocs/op
BenchmarkPing_MetricsOn-8       	  245083	      4588 ns/op	    3152 B/op	      50 allocs/op
BenchmarkPing_MetricsOn-8       	  301550	      4099 ns/op	    3152 B/op	      50 allocs/op
BenchmarkPing_MetricsOn-8       	  313803	      4010 ns/op	    3152 B/op	      50 allocs/op
BenchmarkRoute200_Stdlib-8      	 1509069	       724.3 ns/op	    1032 B/op	      11 allocs/op
BenchmarkRoute200_Stdlib-8      	 1594714	       729.0 ns/op	    1032 B/op	      11 allocs/op
BenchmarkRoute200_Stdlib-8      	 1650973	       734.3 ns/op	    1032 B/op	      11 allocs/op
BenchmarkRoute200_Trilha-8      	  291604	      4210 ns/op	    3168 B/op	      51 allocs/op
BenchmarkRoute200_Trilha-8      	  301279	      3966 ns/op	    3168 B/op	      51 allocs/op
BenchmarkRoute200_Trilha-8      	  308371	      4519 ns/op	    3168 B/op	      51 allocs/op
BenchmarkStatic_Stdlib-8        	  797343	      1479 ns/op	    3916 B/op	      16 allocs/op
BenchmarkStatic_Stdlib-8        	  806536	      1477 ns/op	    3916 B/op	      16 allocs/op
BenchmarkStatic_Stdlib-8        	  872559	      1451 ns/op	    3916 B/op	      16 allocs/op
BenchmarkStatic_Trilha-8        	  261276	      4509 ns/op	    7072 B/op	      60 allocs/op
BenchmarkStatic_Trilha-8        	  262636	      4627 ns/op	    7072 B/op	      60 allocs/op
BenchmarkStatic_Trilha-8        	  270396	      5179 ns/op	    7072 B/op	      60 allocs/op
```
