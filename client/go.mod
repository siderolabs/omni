module github.com/siderolabs/omni/client

go 1.22.4

replace (
	// forked go-yaml that introduces RawYAML interface, which can be used to populate YAML fields using bytes
	// which are then encoded as a valid YAML blocks with proper indentiation
	gopkg.in/yaml.v3 => github.com/unix4ever/yaml v0.0.0-20220527175918-f17b0f05cf2c
	// our fork for tcpproxy with fixes
	inet.af/tcpproxy => github.com/smira/tcpproxy v0.0.0-20200125044825-b6bb9b5b8252
)

require (
	github.com/adrg/xdg v0.4.0
	github.com/blang/semver v3.5.1+incompatible
	github.com/cosi-project/runtime v0.5.2
	github.com/fatih/color v1.17.0
	github.com/google/uuid v1.6.0
	github.com/gosuri/uiprogress v0.0.1
	github.com/grpc-ecosystem/grpc-gateway/v2 v2.20.0
	github.com/hashicorp/go-multierror v1.1.1
	github.com/hexops/gotextdiff v1.0.3
	github.com/mattn/go-isatty v0.0.20
	github.com/planetscale/vtprotobuf v0.6.0
	github.com/sergi/go-diff v1.3.1
	github.com/siderolabs/gen v0.5.0
	github.com/siderolabs/go-api-signature v0.3.3
	github.com/siderolabs/go-kubeconfig v0.1.0
	github.com/siderolabs/go-pointer v1.0.0
	github.com/siderolabs/talos/pkg/machinery v1.7.5
	github.com/spf13/cobra v1.8.1
	github.com/stretchr/testify v1.9.0
	github.com/xlab/treeprint v1.2.0
	go.uber.org/zap v1.27.0
	golang.org/x/sync v0.7.0
	golang.org/x/term v0.21.0
	google.golang.org/grpc v1.65.0
	google.golang.org/protobuf v1.34.2
	gopkg.in/yaml.v3 v3.0.1
	k8s.io/client-go v0.30.2
)

require (
	github.com/ProtonMail/go-crypto v1.0.0 // indirect
	github.com/ProtonMail/go-mime v0.0.0-20230322103455-7d82a3887f2f // indirect
	github.com/ProtonMail/gopenpgp/v2 v2.7.5 // indirect
	github.com/cloudflare/circl v1.3.9 // indirect
	github.com/containerd/go-cni v1.1.9 // indirect
	github.com/containernetworking/cni v1.1.2 // indirect
	github.com/cpuguy83/go-md2man/v2 v2.0.4 //indirect
	github.com/davecgh/go-spew v1.1.1 // indirect
	github.com/dustin/go-humanize v1.0.1 // indirect
	github.com/gertd/go-pluralize v0.2.1 // indirect
	github.com/go-logr/logr v1.4.1 // indirect
	github.com/gogo/protobuf v1.3.2 // indirect
	github.com/google/go-cmp v0.6.0 // indirect
	github.com/google/gofuzz v1.2.0 // indirect
	github.com/gosuri/uilive v0.0.4 // indirect
	github.com/hashicorp/errwrap v1.1.0 // indirect
	github.com/imdario/mergo v0.3.16 // indirect
	github.com/inconshreveable/mousetrap v1.1.0 // indirect
	github.com/josharian/native v1.1.0 // indirect
	github.com/jsimonetti/rtnetlink v1.4.1 // indirect
	github.com/json-iterator/go v1.1.12 // indirect
	github.com/mattn/go-colorable v0.1.13 // indirect
	github.com/mdlayher/ethtool v0.1.0 // indirect
	github.com/mdlayher/genetlink v1.3.2 // indirect
	github.com/mdlayher/netlink v1.7.2 // indirect
	github.com/mdlayher/socket v0.5.1 // indirect
	github.com/modern-go/concurrent v0.0.0-20180306012644-bacd9c7ef1dd // indirect
	github.com/modern-go/reflect2 v1.0.2 // indirect
	github.com/opencontainers/runtime-spec v1.2.0 // indirect
	github.com/pkg/browser v0.0.0-20240102092130-5ac0b6a4141c // indirect
	github.com/pkg/errors v0.9.1 // indirect
	github.com/pmezard/go-difflib v1.0.0 // indirect
	github.com/russross/blackfriday/v2 v2.1.0 // indirect
	github.com/ryanuber/go-glob v1.0.0 // indirect
	github.com/siderolabs/crypto v0.4.4 // indirect
	github.com/siderolabs/go-blockdevice v0.4.7 // indirect
	github.com/siderolabs/net v0.4.0 // indirect
	github.com/siderolabs/protoenc v0.2.1 // indirect
	github.com/spf13/pflag v1.0.5 // indirect
	go.uber.org/multierr v1.11.0 // indirect
	golang.org/x/crypto v0.24.0 // indirect
	golang.org/x/net v0.26.0 // indirect
	golang.org/x/oauth2 v0.20.0 // indirect
	golang.org/x/sys v0.21.0 // indirect
	golang.org/x/text v0.16.0 // indirect
	golang.org/x/time v0.5.0 // indirect
	google.golang.org/genproto/googleapis/api v0.0.0-20240617180043-68d350f18fd4 // indirect
	google.golang.org/genproto/googleapis/rpc v0.0.0-20240617180043-68d350f18fd4 // indirect
	gopkg.in/inf.v0 v0.9.1 // indirect
	gopkg.in/yaml.v2 v2.4.0 // indirect
	k8s.io/apimachinery v0.30.2 // indirect
	k8s.io/klog/v2 v2.120.1 // indirect
	k8s.io/utils v0.0.0-20231127182322-b307cd553661 // indirect
	sigs.k8s.io/json v0.0.0-20221116044647-bc3834ca7abd // indirect
	sigs.k8s.io/structured-merge-diff/v4 v4.4.1 // indirect
	sigs.k8s.io/yaml v1.4.0 // indirect
)
