module github.com/siderolabs/omni/client

go 1.24.0

replace (
	// forked go-yaml that introduces RawYAML interface, which can be used to populate YAML fields using bytes
	// which are then encoded as a valid YAML blocks with proper indentiation
	gopkg.in/yaml.v3 => github.com/unix4ever/yaml/v2 v2.4.0
	// our fork for tcpproxy with fixes
	inet.af/tcpproxy => github.com/smira/tcpproxy v0.0.0-20200125044825-b6bb9b5b8252
)

require (
	github.com/ProtonMail/gopenpgp/v3 v3.1.3
	github.com/adrg/xdg v0.5.3
	github.com/blang/semver v3.5.1+incompatible
	github.com/blang/semver/v4 v4.0.0
	github.com/cosi-project/runtime v0.10.1
	github.com/dustin/go-humanize v1.0.1
	github.com/fatih/color v1.18.0
	github.com/gertd/go-pluralize v0.2.1
	github.com/google/uuid v1.6.0
	github.com/gosuri/uiprogress v0.0.1
	github.com/grpc-ecosystem/grpc-gateway/v2 v2.26.3
	github.com/hashicorp/go-multierror v1.1.1
	github.com/hexops/gotextdiff v1.0.3
	github.com/klauspost/compress v1.18.0
	github.com/mattn/go-isatty v0.0.20
	github.com/planetscale/vtprotobuf ba97887b0a25
	github.com/sergi/go-diff v1.3.1
	github.com/siderolabs/gen v0.8.0
	github.com/siderolabs/go-api-signature v0.3.6
	github.com/siderolabs/go-kubeconfig v0.1.1
	github.com/siderolabs/go-pointer v1.0.1
	github.com/siderolabs/image-factory v0.6.7
	github.com/siderolabs/proto-codec v0.1.2
	github.com/siderolabs/talos/pkg/machinery v1.10.0-alpha.2
	github.com/spf13/cobra v1.9.1
	github.com/stretchr/testify v1.10.0
	github.com/xlab/treeprint v1.2.0
	go.uber.org/zap v1.27.0
	golang.org/x/sync v0.12.0
	golang.org/x/term v0.30.0
	google.golang.org/grpc v1.71.0
	google.golang.org/protobuf v1.36.5
	gopkg.in/yaml.v3 v3.0.1
	k8s.io/client-go v0.32.3
)

require (
	cel.dev/expr v0.19.1 // indirect
	github.com/ProtonMail/go-crypto v1.1.5 // indirect
	github.com/ProtonMail/go-mime v0.0.0-20230322103455-7d82a3887f2f // indirect
	github.com/antlr4-go/antlr/v4 v4.13.1 // indirect
	github.com/cenkalti/backoff/v4 v4.3.0 // indirect
	github.com/cloudflare/circl v1.5.0 // indirect
	github.com/containerd/go-cni v1.1.12 // indirect
	github.com/containernetworking/cni v1.2.3 // indirect
	github.com/cpuguy83/go-md2man/v2 v2.0.6 //indirect
	github.com/davecgh/go-spew v1.1.2-0.20180830191138-d8f796af33cc // indirect
	github.com/fxamacker/cbor/v2 v2.7.0 // indirect
	github.com/go-logr/logr v1.4.2 // indirect
	github.com/gogo/protobuf v1.3.2 // indirect
	github.com/google/cel-go v0.22.1 // indirect
	github.com/google/go-cmp v0.6.0 // indirect
	github.com/google/gofuzz v1.2.0 // indirect
	github.com/gosuri/uilive v0.0.4 // indirect
	github.com/hashicorp/errwrap v1.1.0 // indirect
	github.com/inconshreveable/mousetrap v1.1.0 // indirect
	github.com/josharian/native v1.1.0 // indirect
	github.com/jsimonetti/rtnetlink/v2 v2.0.3-0.20241216183107-2d6e9f8ad3f2 // indirect
	github.com/json-iterator/go v1.1.12 // indirect
	github.com/mattn/go-colorable v0.1.14 // indirect
	github.com/mdlayher/ethtool v0.2.0 // indirect
	github.com/mdlayher/genetlink v1.3.2 // indirect
	github.com/mdlayher/netlink v1.7.2 // indirect
	github.com/mdlayher/socket v0.5.1 // indirect
	github.com/modern-go/concurrent v0.0.0-20180306012644-bacd9c7ef1dd // indirect
	github.com/modern-go/reflect2 v1.0.2 // indirect
	github.com/munnerz/goautoneg v0.0.0-20191010083416-a7dc8b61c822 // indirect
	github.com/opencontainers/runtime-spec v1.2.0 // indirect
	github.com/petermattis/goid v0.0.0-20241211131331-93ee7e083c43 // indirect
	github.com/pkg/browser v0.0.0-20240102092130-5ac0b6a4141c // indirect
	github.com/pkg/errors v0.9.1 // indirect
	github.com/pmezard/go-difflib v1.0.1-0.20181226105442-5d4384ee4fb2 // indirect
	github.com/russross/blackfriday/v2 v2.1.0 // indirect
	github.com/ryanuber/go-glob v1.0.0 // indirect
	github.com/sasha-s/go-deadlock v0.3.5 // indirect
	github.com/siderolabs/crypto v0.5.1 // indirect
	github.com/siderolabs/go-blockdevice/v2 v2.0.13 // indirect
	github.com/siderolabs/net v0.4.0 // indirect
	github.com/siderolabs/protoenc v0.2.2 // indirect
	github.com/spf13/pflag v1.0.5 // indirect
	github.com/stoewer/go-strcase v1.3.0 // indirect
	github.com/x448/float16 v0.8.4 // indirect
	go.uber.org/multierr v1.11.0 // indirect
	golang.org/x/crypto v0.32.0 // indirect
	golang.org/x/exp v0.0.0-20250106191152-7588d65b2ba8 // indirect
	golang.org/x/net v0.34.0 // indirect
	golang.org/x/oauth2 v0.25.0 // indirect
	golang.org/x/sys v0.30.0 // indirect
	golang.org/x/text v0.21.0 // indirect
	golang.org/x/time v0.10.0 // indirect
	google.golang.org/genproto/googleapis/api v0.0.0-20250127172529-29210b9bc287 // indirect
	google.golang.org/genproto/googleapis/rpc v0.0.0-20250127172529-29210b9bc287 // indirect
	gopkg.in/inf.v0 v0.9.1 // indirect
	k8s.io/apimachinery v0.32.0 // indirect
	k8s.io/klog/v2 v2.130.1 // indirect
	k8s.io/utils v0.0.0-20241210054802-24370beab758 // indirect
	sigs.k8s.io/json v0.0.0-20241014173422-cfa47c3a1cc8 // indirect
	sigs.k8s.io/structured-merge-diff/v4 v4.5.0 // indirect
	sigs.k8s.io/yaml v1.4.0 // indirect
)
