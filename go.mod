module github.com/uidops/pure-market-maker

go 1.22.5

require github.com/uidops/pure-market-maker/exchanges v0.0.0

replace github.com/uidops/pure-market-maker/exchanges => ./exchanges

require github.com/uidops/pure-market-maker/exchanges/mexc v0.0.0

replace github.com/uidops/pure-market-maker/exchanges/mexc => ./exchanges/mexc
