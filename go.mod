module oldbeggar-refactor

go 1.22

require (
	github.com/Luzifer/go-openssl/v4 v4.2.2
	github.com/advancedclimatesystems/gonnx v1.1.0
	github.com/go-sql-driver/mysql v1.7.1
	github.com/nfnt/resize v0.0.0-20180221191011-83c6a9932646
	github.com/robfig/cron/v3 v3.0.1
	golang.org/x/image v0.6.0
	gopkg.in/yaml.v3 v3.0.1
	gorgonia.org/tensor v0.9.24
)

require (
	github.com/apache/arrow/go/arrow v0.0.0-20211112161151-bc219186db40 // indirect
	github.com/chewxy/hm v1.0.0 // indirect
	github.com/chewxy/math32 v1.10.1 // indirect
	github.com/gogo/protobuf v1.3.2 // indirect
	github.com/golang/protobuf v1.5.3 // indirect
	github.com/google/flatbuffers v23.5.26+incompatible // indirect
	github.com/pkg/errors v0.9.1 // indirect
	github.com/xtgo/set v1.0.0 // indirect
	go4.org/unsafe/assume-no-moving-gc v0.0.0-20231121144256-b99613f794b6 // indirect
	golang.org/x/crypto v0.17.0 // indirect
	golang.org/x/xerrors v0.0.0-20231012003039-104605ab7028 // indirect
	gonum.org/v1/gonum v0.14.0 // indirect
	google.golang.org/protobuf v1.31.0 // indirect
	gorgonia.org/vecf32 v0.9.0 // indirect
	gorgonia.org/vecf64 v0.9.0 // indirect
)

replace github.com/advancedclimatesystems/gonnx => ./third_party/gonnx
