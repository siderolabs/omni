module github.com/siderolabs/omni

go 1.24.0

replace (
	// use nested module
	github.com/siderolabs/omni/client => ./client
	// forked go-yaml that introduces RawYAML interface, which can be used to populate YAML fields using bytes
	// which are then encoded as a valid YAML blocks with proper indentiation
	gopkg.in/yaml.v3 => github.com/unix4ever/yaml/v2 v2.4.0
)

require (
	filippo.io/age v1.2.1
	github.com/ProtonMail/gopenpgp/v3 v3.1.3
	github.com/adrg/xdg v0.5.3
	github.com/akutz/memconn v0.1.1-0.20211110233653-dae351d188b3
	github.com/auth0/go-jwt-middleware/v2 v2.3.0
	github.com/aws/aws-sdk-go-v2 v1.36.3
	github.com/aws/aws-sdk-go-v2/config v1.29.9
	github.com/aws/aws-sdk-go-v2/credentials v1.17.62
	github.com/aws/aws-sdk-go-v2/feature/s3/manager v1.17.66
	github.com/aws/aws-sdk-go-v2/service/s3 v1.78.2
	github.com/aws/smithy-go v1.22.3
	github.com/benbjohnson/clock v1.3.5
	github.com/blang/semver v3.5.1+incompatible
	github.com/blang/semver/v4 v4.0.0
	github.com/cenkalti/backoff/v5 v5.0.2
	github.com/containers/image/v5 v5.34.2
	github.com/cosi-project/runtime v0.10.1
	github.com/cosi-project/state-etcd v0.5.1
	github.com/crewjam/saml v0.4.14
	github.com/dustin/go-humanize v1.0.1
	github.com/emicklei/dot v1.8.0
	github.com/felixge/httpsnoop v1.0.4
	github.com/fsnotify/fsnotify v1.8.0
	github.com/gertd/go-pluralize v0.2.1
	github.com/go-jose/go-jose/v4 v4.0.4
	github.com/go-logr/zapr v1.3.0
	github.com/golang-jwt/jwt/v5 v5.2.1
	github.com/google/go-cmp v0.7.0
	github.com/google/go-containerregistry v0.20.3
	github.com/google/uuid v1.6.0
	github.com/grpc-ecosystem/go-grpc-middleware/v2 v2.3.1
	github.com/grpc-ecosystem/go-grpc-prometheus v1.2.0
	github.com/grpc-ecosystem/grpc-gateway/v2 v2.26.3
	github.com/hashicorp/go-cleanhttp v0.5.2
	github.com/hashicorp/go-multierror v1.1.1
	github.com/hashicorp/golang-lru/v2 v2.0.7
	github.com/hashicorp/vault/api v1.16.0
	github.com/hashicorp/vault/api/auth/kubernetes v0.9.0
	github.com/hexops/gotextdiff v1.0.3
	github.com/jellydator/ttlcache/v3 v3.3.0
	github.com/johannesboyne/gofakes3 5c39aecd6999
	github.com/julienschmidt/httprouter v1.3.0
	github.com/jxskiss/base62 v1.1.0
	github.com/klauspost/compress v1.18.0
	github.com/mattn/go-shellwords v1.0.12
	github.com/prometheus/client_golang v1.21.1
	github.com/prometheus/common v0.63.0
	github.com/santhosh-tekuri/jsonschema/v6 v6.0.1
	github.com/siderolabs/crypto v0.5.1
	github.com/siderolabs/discovery-api v0.1.6
	github.com/siderolabs/discovery-client v0.1.11
	github.com/siderolabs/discovery-service v1.0.10
	github.com/siderolabs/gen v0.8.0
	github.com/siderolabs/go-api-signature v0.3.6
	github.com/siderolabs/go-circular v0.2.2
	github.com/siderolabs/go-debug v0.5.0
	github.com/siderolabs/go-kubernetes v0.2.20
	github.com/siderolabs/go-loadbalancer v0.4.0
	github.com/siderolabs/go-pointer v1.0.1
	github.com/siderolabs/go-procfs v0.1.2
	github.com/siderolabs/go-retry v0.3.3
	github.com/siderolabs/go-tail v0.1.1
	github.com/siderolabs/go-talos-support v0.1.2
	github.com/siderolabs/grpc-proxy v0.5.1
	github.com/siderolabs/image-factory v0.6.7
	github.com/siderolabs/kms-client v0.1.0
	github.com/siderolabs/omni/client v0.47.1
	github.com/siderolabs/proto-codec v0.1.2
	github.com/siderolabs/siderolink v0.3.13
	github.com/siderolabs/talos/pkg/machinery v1.10.0-alpha.2
	github.com/sirupsen/logrus v1.9.3
	github.com/spf13/cobra v1.9.1
	github.com/stretchr/testify v1.10.0
	github.com/stripe/stripe-go/v76 v76.25.0
	github.com/zitadel/logging v0.6.1
	github.com/zitadel/oidc/v3 v3.36.1
	go.etcd.io/bbolt v1.4.0
	go.etcd.io/etcd/client/pkg/v3 v3.5.19
	go.etcd.io/etcd/client/v3 v3.5.19
	go.etcd.io/etcd/server/v3 v3.5.19
	go.uber.org/goleak v1.3.0
	go.uber.org/zap v1.27.0
	golang.org/x/crypto v0.36.0
	golang.org/x/net v0.35.0
	golang.org/x/sync v0.12.0
	golang.org/x/time v0.11.0
	golang.org/x/tools v0.31.0
	golang.zx2c4.com/wireguard v0.0.0-20231211153847-12269c276173
	golang.zx2c4.com/wireguard/wgctrl v0.0.0-20241231184526-a9ab2273dd10
	google.golang.org/grpc v1.71.0
	google.golang.org/protobuf v1.36.5
	gopkg.in/yaml.v3 v3.0.3
	k8s.io/api v0.32.3
	k8s.io/apimachinery v0.32.3
	k8s.io/client-go v0.32.3
	k8s.io/klog/v2 v2.130.1
	sigs.k8s.io/controller-runtime v0.20.3
)

require (
	cel.dev/expr v0.19.1 // indirect
	github.com/ProtonMail/go-crypto v1.1.6 // indirect
	github.com/ProtonMail/go-mime v0.0.0-20230322103455-7d82a3887f2f // indirect
	github.com/antlr4-go/antlr/v4 v4.13.1 // indirect
	github.com/aws/aws-sdk-go v1.44.256 // indirect
	github.com/aws/aws-sdk-go-v2/aws/protocol/eventstream v1.6.7 // indirect
	github.com/aws/aws-sdk-go-v2/feature/ec2/imds v1.16.23 // indirect
	github.com/aws/aws-sdk-go-v2/internal/configsources v1.3.27 // indirect
	github.com/aws/aws-sdk-go-v2/internal/endpoints/v2 v2.6.27 // indirect
	github.com/aws/aws-sdk-go-v2/internal/ini v1.8.1 // indirect
	github.com/aws/aws-sdk-go-v2/internal/v4a v1.3.27 // indirect
	github.com/aws/aws-sdk-go-v2/service/internal/accept-encoding v1.12.1 // indirect
	github.com/aws/aws-sdk-go-v2/service/internal/checksum v1.4.8 // indirect
	github.com/aws/aws-sdk-go-v2/service/internal/presigned-url v1.12.8 // indirect
	github.com/aws/aws-sdk-go-v2/service/internal/s3shared v1.18.8 // indirect
	github.com/aws/aws-sdk-go-v2/service/sso v1.24.9 // indirect
	github.com/aws/aws-sdk-go-v2/service/ssooidc v1.28.8 // indirect
	github.com/aws/aws-sdk-go-v2/service/sts v1.33.7 // indirect
	github.com/beevik/etree v1.4.1 // indirect
	github.com/beorn7/perks v1.0.1 // indirect
	github.com/bmatcuk/doublestar/v4 v4.8.0 // indirect
	github.com/cespare/xxhash/v2 v2.3.0 // indirect
	github.com/cloudflare/circl v1.6.0 // indirect
	github.com/containerd/go-cni v1.1.12 // indirect
	github.com/containerd/stargz-snapshotter/estargz v0.16.3 // indirect
	github.com/containernetworking/cni v1.2.3 // indirect
	github.com/containers/storage v1.56.1 // indirect
	github.com/coreos/go-semver v0.3.1 // indirect
	github.com/coreos/go-systemd/v22 v22.5.0 // indirect
	github.com/cpuguy83/go-md2man/v2 v2.0.6 // indirect
	github.com/crewjam/httperr v0.2.0 // indirect
	github.com/davecgh/go-spew v1.1.2-0.20180830191138-d8f796af33cc // indirect
	github.com/docker/cli v27.5.0+incompatible // indirect
	github.com/docker/distribution v2.8.3+incompatible // indirect
	github.com/docker/docker-credential-helpers v0.8.2 // indirect
	github.com/emicklei/go-restful/v3 v3.12.1 // indirect
	github.com/evanphx/json-patch v5.9.0+incompatible // indirect
	github.com/fatih/color v1.18.0 // indirect
	github.com/fxamacker/cbor/v2 v2.7.0 // indirect
	github.com/ghodss/yaml v1.0.0 // indirect
	github.com/go-chi/chi/v5 v5.2.0 // indirect
	github.com/go-logr/logr v1.4.2 // indirect
	github.com/go-logr/stdr v1.2.2 // indirect
	github.com/go-openapi/jsonpointer v0.21.0 // indirect
	github.com/go-openapi/jsonreference v0.21.0 // indirect
	github.com/go-openapi/swag v0.23.0 // indirect
	github.com/gogo/protobuf v1.3.2 // indirect
	github.com/golang/protobuf v1.5.4 // indirect
	github.com/google/btree v1.1.3 // indirect
	github.com/google/cel-go v0.22.1 // indirect
	github.com/google/gnostic-models v0.6.9 // indirect
	github.com/google/gofuzz v1.2.0 // indirect
	github.com/gorilla/securecookie v1.1.2 // indirect
	github.com/gorilla/websocket v1.5.3 // indirect
	github.com/gosuri/uilive v0.0.4 // indirect
	github.com/gosuri/uiprogress v0.0.1 // indirect
	github.com/grpc-ecosystem/go-grpc-middleware/providers/prometheus v1.0.1 // indirect
	github.com/grpc-ecosystem/go-grpc-middleware/v2 v2.2.0 // indirect
	github.com/grpc-ecosystem/grpc-gateway v1.16.0 // indirect
	github.com/hashicorp/errwrap v1.1.0 // indirect
	github.com/hashicorp/go-retryablehttp v0.7.7 // indirect
	github.com/hashicorp/go-rootcerts v1.0.2 // indirect
	github.com/hashicorp/go-secure-stdlib/parseutil v0.1.8 // indirect
	github.com/hashicorp/go-secure-stdlib/strutil v0.1.2 // indirect
	github.com/hashicorp/go-sockaddr v1.0.7 // indirect
	github.com/hashicorp/hcl v1.0.1-vault-7 // indirect
	github.com/inconshreveable/mousetrap v1.1.0 // indirect
	github.com/jonboulle/clockwork v0.5.0 // indirect
	github.com/josharian/intern v1.0.0 // indirect
	github.com/josharian/native v1.1.0 // indirect
	github.com/jsimonetti/rtnetlink/v2 v2.0.3-0.20241216183107-2d6e9f8ad3f2 // indirect
	github.com/json-iterator/go v1.1.12 // indirect
	github.com/mailru/easyjson v0.9.0 // indirect
	github.com/mattermost/xml-roundtrip-validator v0.1.0 // indirect
	github.com/mattn/go-colorable v0.1.14 // indirect
	github.com/mattn/go-isatty v0.0.20 // indirect
	github.com/mdlayher/ethtool v0.2.0 // indirect
	github.com/mdlayher/genetlink v1.3.2 // indirect
	github.com/mdlayher/netlink v1.7.2 // indirect
	github.com/mdlayher/socket v0.5.1 // indirect
	github.com/mitchellh/go-homedir v1.1.0 // indirect
	github.com/mitchellh/mapstructure v1.5.0 // indirect
	github.com/modern-go/concurrent v0.0.0-20180306012644-bacd9c7ef1dd // indirect
	github.com/modern-go/reflect2 v1.0.2 // indirect
	github.com/muhlemmer/gu v0.3.1 // indirect
	github.com/muhlemmer/httpforwarded v0.1.0 // indirect
	github.com/munnerz/goautoneg v0.0.0-20191010083416-a7dc8b61c822 // indirect
	github.com/opencontainers/go-digest v1.0.0 // indirect
	github.com/opencontainers/image-spec v1.1.0 // indirect
	github.com/opencontainers/runtime-spec v1.2.0 // indirect
	github.com/petermattis/goid v0.0.0-20241211131331-93ee7e083c43 // indirect
	github.com/pkg/browser v0.0.0-20240102092130-5ac0b6a4141c // indirect
	github.com/pkg/errors v0.9.1 // indirect
	github.com/planetscale/vtprotobuf v0.6.1-0.20241121165744-79df5c4772f2 // indirect
	github.com/pmezard/go-difflib v1.0.1-0.20181226105442-5d4384ee4fb2 // indirect
	github.com/prometheus/client_model v0.6.1 // indirect
	github.com/prometheus/procfs v0.15.1 // indirect
	github.com/rs/cors v1.11.1 // indirect
	github.com/russellhaering/goxmldsig v1.4.0 // indirect
	github.com/russross/blackfriday/v2 v2.1.0 // indirect
	github.com/ryanuber/go-glob v1.0.0 // indirect
	github.com/ryszard/goskiplist v0.0.0-20150312221310-2dfbae5fcf46 // indirect
	github.com/sasha-s/go-deadlock v0.3.5 // indirect
	github.com/sergi/go-diff v1.3.1 // indirect
	github.com/siderolabs/go-blockdevice/v2 v2.0.13 // indirect
	github.com/siderolabs/go-kubeconfig v0.1.0 // indirect
	github.com/siderolabs/net v0.4.0 // indirect
	github.com/siderolabs/protoenc v0.2.2 // indirect
	github.com/siderolabs/tcpproxy v0.1.0 // indirect
	github.com/soheilhy/cmux v0.1.5 // indirect
	github.com/spf13/pflag v1.0.6 // indirect
	github.com/stoewer/go-strcase v1.3.0 // indirect
	github.com/stretchr/objx v0.5.2 // indirect
	github.com/tmc/grpc-websocket-proxy v0.0.0-20220101234140-673ab2c3ae75 // indirect
	github.com/vbatts/tar-split v0.11.6 // indirect
	github.com/x448/float16 v0.8.4 // indirect
	github.com/xiang90/probing v0.0.0-20221125231312-a49e3df8f510 // indirect
	github.com/xlab/treeprint v1.2.0 // indirect
	github.com/zitadel/schema v1.3.0 // indirect
	go.etcd.io/etcd/api/v3 v3.5.18 // indirect
	go.etcd.io/etcd/client/v2 v2.305.18 // indirect
	go.etcd.io/etcd/pkg/v3 v3.5.18 // indirect
	go.etcd.io/etcd/raft/v3 v3.5.18 // indirect
	go.opentelemetry.io/auto/sdk v1.1.0 // indirect
	go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc v0.59.0 // indirect
	go.opentelemetry.io/otel v1.34.0 // indirect
	go.opentelemetry.io/otel/exporters/otlp/otlptrace v1.34.0 // indirect
	go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc v1.34.0 // indirect
	go.opentelemetry.io/otel/metric v1.34.0 // indirect
	go.opentelemetry.io/otel/sdk v1.34.0 // indirect
	go.opentelemetry.io/otel/trace v1.34.0 // indirect
	go.opentelemetry.io/proto/otlp v1.5.0 // indirect
	go.shabbyrobe.org/gocovmerge v0.0.0-20230507111327-fa4f82cfbf4d // indirect
	go.uber.org/multierr v1.11.0 // indirect
	go4.org/netipx v0.0.0-20231129151722-fdeea329fbba // indirect
	golang.org/x/exp v0.0.0-20250106191152-7588d65b2ba8 // indirect
	golang.org/x/mod v0.22.0 // indirect
	golang.org/x/oauth2 v0.26.0 // indirect
	golang.org/x/sys v0.30.0 // indirect
	golang.org/x/term v0.29.0 // indirect
	golang.org/x/text v0.22.0 // indirect
	golang.zx2c4.com/wintun v0.0.0-20230126152724-0fa3db229ce2 // indirect
	google.golang.org/genproto v0.0.0-20250204164813-702378808489 // indirect
	google.golang.org/genproto/googleapis/api v0.0.0-20250227231956-55c901821b1e // indirect
	google.golang.org/genproto/googleapis/rpc v0.0.0-20250227231956-55c901821b1e // indirect
	gopkg.in/evanphx/json-patch.v4 v4.12.0 // indirect
	gopkg.in/go-jose/go-jose.v2 v2.6.3 // indirect
	gopkg.in/inf.v0 v0.9.1 // indirect
	gopkg.in/natefinch/lumberjack.v2 v2.2.1 // indirect
	gopkg.in/yaml.v2 v2.4.0 // indirect
	k8s.io/kube-openapi v0.0.0-20241212222426-2c72e554b1e7 // indirect
	k8s.io/utils v0.0.0-20241210054802-24370beab758 // indirect
	sigs.k8s.io/json v0.0.0-20241014173422-cfa47c3a1cc8 // indirect
	sigs.k8s.io/structured-merge-diff/v4 v4.5.0 // indirect
	sigs.k8s.io/yaml v1.4.0 // indirect
)
