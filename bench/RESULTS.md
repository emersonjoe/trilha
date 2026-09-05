# Resultados de referência

Gerado em 2026-09-05 por `make bench-results` — go1.25.1, Apple M2, Darwin arm64.
Custo por requisição em processo (`httptest`), só o framework; leia a metodologia em
https://emersonjoe.github.io/trilha/referencia/desempenho.

```
BenchmarkJSON_Stdlib-8          	  252228	      4217 ns/op	    4428 B/op	      10 allocs/op
BenchmarkJSON_Stdlib-8          	  284434	      4202 ns/op	    4428 B/op	      10 allocs/op
BenchmarkJSON_Stdlib-8          	  288706	      4530 ns/op	    4428 B/op	      10 allocs/op
BenchmarkJSON_Trilha-8          	  154850	      7707 ns/op	    6545 B/op	      51 allocs/op
BenchmarkJSON_Trilha-8          	  160149	      7540 ns/op	    6545 B/op	      51 allocs/op
BenchmarkJSON_Trilha-8          	  161511	      7585 ns/op	    6544 B/op	      51 allocs/op
BenchmarkMiddleware5_Stdlib-8   	 1714981	       642.9 ns/op	    1016 B/op	      10 allocs/op
BenchmarkMiddleware5_Stdlib-8   	 1849366	       636.9 ns/op	    1016 B/op	      10 allocs/op
BenchmarkMiddleware5_Stdlib-8   	 1859859	       645.4 ns/op	    1016 B/op	      10 allocs/op
BenchmarkMiddleware5_Trilha-8   	  269886	      4082 ns/op	    3408 B/op	      51 allocs/op
BenchmarkMiddleware5_Trilha-8   	  287760	      3957 ns/op	    3408 B/op	      51 allocs/op
BenchmarkMiddleware5_Trilha-8   	  304399	      4188 ns/op	    3408 B/op	      51 allocs/op
BenchmarkPage_Stdlib-8          	   36270	     31252 ns/op	   18944 B/op	     270 allocs/op
BenchmarkPage_Stdlib-8          	   41583	     29389 ns/op	   18944 B/op	     270 allocs/op
BenchmarkPage_Stdlib-8          	   42110	     29403 ns/op	   18944 B/op	     270 allocs/op
BenchmarkPage_Trilha-8          	   55473	     20098 ns/op	   28209 B/op	     482 allocs/op
BenchmarkPage_Trilha-8          	   62151	     19447 ns/op	   28209 B/op	     482 allocs/op
BenchmarkPage_Trilha-8          	   63698	     19365 ns/op	   28209 B/op	     482 allocs/op
BenchmarkRoute200_Stdlib-8      	 1686613	       720.2 ns/op	    1032 B/op	      11 allocs/op
BenchmarkRoute200_Stdlib-8      	 1689512	       714.4 ns/op	    1032 B/op	      11 allocs/op
BenchmarkRoute200_Stdlib-8      	 1694019	       724.0 ns/op	    1032 B/op	      11 allocs/op
BenchmarkRoute200_Trilha-8      	  306260	      4088 ns/op	    3136 B/op	      51 allocs/op
BenchmarkRoute200_Trilha-8      	  319446	      3994 ns/op	    3136 B/op	      51 allocs/op
BenchmarkRoute200_Trilha-8      	  322024	      3987 ns/op	    3136 B/op	      51 allocs/op
BenchmarkStatic_Stdlib-8        	  828883	      1406 ns/op	    3916 B/op	      16 allocs/op
BenchmarkStatic_Stdlib-8        	  871815	      1401 ns/op	    3916 B/op	      16 allocs/op
BenchmarkStatic_Stdlib-8        	  873525	      1430 ns/op	    3916 B/op	      16 allocs/op
BenchmarkStatic_Trilha-8        	  263904	      4517 ns/op	    7040 B/op	      60 allocs/op
BenchmarkStatic_Trilha-8        	  271580	      4343 ns/op	    7040 B/op	      60 allocs/op
BenchmarkStatic_Trilha-8        	  280858	      4299 ns/op	    7040 B/op	      60 allocs/op
```
