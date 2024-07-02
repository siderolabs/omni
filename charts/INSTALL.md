# Certificate
Omni will require valid SSL certificates. This means that self-signed certs will not work as of the time of this writing.</br>
Here is an example of a manifest to create a Certificate with Cert-Manager.

```shell
apiVersion: cert-manager.io/v1
kind: Certificate
metadata:
  name: <omni-domain>-cert
spec:
  dnsNames:
    - <omni-domain>
  issuerRef:
    name: letsencrypt
    kind: ClusterIssuer
  secretName: <omni-domain>-tls
```

# ETCD Encryption Key
Omni uses an encrypted ETCD database to store it's information. There for it needs a encryption key.<br>
We are creating a new encrypted key a store in a file and create a Kubernetes secret of that file.

```shell
# Generate Key
export EMAIL=info@company.local
gpg --batch --passphrase ''  --quick-generate-key "Omni (Used for etcd data encryption) $EMAIL" rsa4096 cert never
TUMB=$(gpg --list-keys "Omni (Used for etcd data encryption) $EMAIL" | head -n 2 | tail -n 1 | tr -d [:space:])
gpg --batch --passphrase '' --quick-add-key $TUMB rsa4096 encr never
gpg --batch --passphrase '' --export-secret-key --armor $EMAIL > ./omni.asc
```
```shell
# Create Kubernetes Secret
kubectl create secret generic omni-etcd-key --from-file=omni.asc
```
> **Note**: Do not add passphrases to keys during creation.

### Account ID
It is important to generate a unique ID for this Omni deployment. It will also be necessary to use this same UUID each time you start your Omni instance.

```shell
# Generate a UUID
~$ uuidgen 
4B424EC2-023B-4710-BC1F-4AEB9C214C56
```
```shell
# Create Kubernetes ConfigMap
kubectl create configmap omni-account-uuid \
  --from-literal uuid=4B424EC2-023B-4710-BC1F-4AEB9C214C56
```

### Deployment
Everything is now inplace, let's deploy Omni!

#### Helm
<details>
<summary>Example: omni_values.yaml</summary>

```shell
# -- Number of pods of the deployment (only applies for Omni Deployment)
replicaCount: 1

omniAccount:
  existingConfigMap: "omni-account-uuid"

tls:
  # -- Domain that is used by Omni
  domain: "<omni-domain>"
  # -- existing Secret that contains the certificate and key
  existingTlsSecret: "<omni-domain>-tls"

etcd:
  # -- existing Secret that contains the ETCD encryption key
  encryptionKeySecret: "omni-etcd-key"

authentication:
  # -- There are 2 option for authentication. saml or auth0
  # -- Which authentication type
  type: saml
  saml:
    # -- SAML URL
    url: "https://<keycloak-domain>/realms/omni/protocol/saml/descriptor"

wireguard:
  # -- IP Address of the service that exposes wireguard
  ipAddr: "<ipaddress of the LoadBalancer>"

service:
  # -- IP Address of the LoadBalancer
  loadBalancerIP: "<predefined ipadress>"

```
</details></br>

```shell
# Clone gitRepo
git clone git@github.com:siderolabs/omni.git
```
```shell
# Install Omni
helm install \
  omni charts/omni \
  --version v1.0.0 \
  --values omni_values.yaml
```
