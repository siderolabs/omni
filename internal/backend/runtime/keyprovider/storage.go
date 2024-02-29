// Copyright (c) 2024 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

// Package keyprovider contains a key provider.
package keyprovider

import (
	"context"
	"crypto/rand"
	"errors"

	"github.com/ProtonMail/gopenpgp/v2/crypto"
	"github.com/cosi-project/runtime/pkg/keystorage"
	"github.com/cosi-project/state-etcd/pkg/keystorage/recstore"
	"github.com/cosi-project/state-etcd/pkg/state/impl/etcd"
	"github.com/siderolabs/gen/ensure"
	"github.com/siderolabs/gen/xerrors"
	"go.uber.org/zap"
)

// KeyProvider is a key provider.
type KeyProvider struct {
	recstore   *recstore.RecStore[*keystorage.KeyStorage]
	privateKey PrivateKeyData
	version    int64
}

// ProvideKey imlements KeyProvider.
func (s *KeyProvider) ProvideKey() ([]byte, error) {
	got, err := s.recstore.Get(context.Background())
	if err != nil {
		return nil, err
	}

	key, err := got.Res.GetMasterKey(s.privateKey.slot, s.privateKey.key)
	if err != nil {
		return nil, err
	}

	return key, nil
}

// New creates a new key provider.
func New(
	client etcd.Client,
	name string,
	privateKey PrivateKeyData,
	publicKeys []PublicKeyData,
	logger *zap.Logger,
) (*KeyProvider, error) {
	recStore := recstore.New(client, name, "keystore-omni", storageMarshal, storageUnmarshal)

	res, err := recStore.Get(context.Background())
	if err != nil {
		ok := xerrors.TagIs[recstore.NotFoundTag](err)
		if !ok {
			return nil, err
		}

		// We do not have a key store yet, so we create a new one.
		return initKeyProvider(name, recStore, privateKey, publicKeys, logger)
	}

	keyStorage := res.Res
	version := res.Version

	_, err = keyStorage.GetMasterKey(privateKey.slot, privateKey.key)
	if err != nil {
		return nil, err
	}

	if len(publicKeys) > 0 {
		// We have new public keys, let's try to add them to the key storage
		err = storePublicKeys(keyStorage, privateKey, publicKeys, logger)
		if err != nil {
			return nil, err
		}

		err = recStore.Update(context.Background(), keyStorage, version)
		if err != nil {
			return nil, err
		}

		version++
	}

	logger.Info(
		"using existing key store",
		zap.String("name", name),
		zap.String("slot", privateKey.slot),
		zap.Int64("version", version),
		zap.String("key_identity", ensure.Value(GetIdentityString(privateKey.key))),
	)

	return &KeyProvider{
		recstore:   recStore,
		privateKey: privateKey,
		version:    version,
	}, nil
}

// GetIdentityString returns the identity string from the openpgp key.
func GetIdentityString(key string) (string, error) {
	armored, err := crypto.NewKeyFromArmored(key)
	if err != nil {
		return "", err
	}

	firstIdentity := ""

	for name := range armored.GetEntity().Identities {
		firstIdentity = name

		break
	}

	if firstIdentity == "" {
		return "", errors.New("no identity attached to the key found")
	}

	return firstIdentity + " <" + armored.GetFingerprint() + ">", nil
}

func storePublicKeys(ks *keystorage.KeyStorage, privateKey PrivateKeyData, publicKeys []PublicKeyData, logger *zap.Logger) error {
	for _, elem := range publicKeys {
		err := ks.AddKeySlot(elem.slot, elem.key, privateKey.slot, privateKey.key)
		if err != nil {
			if !xerrors.TagIs[keystorage.SlotAlreadyExists](err) {
				return err
			}

			// Ignore SlotAlreadyExists error
			logger.Info("slot already exists, skipping", zap.String("slot", elem.slot))

			continue
		}
	}

	return nil
}

func storageMarshal(ks *keystorage.KeyStorage) ([]byte, error) {
	return ks.MarshalBinary()
}

func storageUnmarshal(data []byte) (*keystorage.KeyStorage, error) {
	ks := &keystorage.KeyStorage{}

	if err := ks.UnmarshalBinary(data); err != nil {
		return nil, err
	}

	return ks, nil
}

func initKeyProvider(name string, recStore *recstore.RecStore[*keystorage.KeyStorage], privateKey PrivateKeyData, publicKeys []PublicKeyData, logger *zap.Logger) (*KeyProvider, error) {
	logger.Warn(
		"initializing new key store",
		zap.String("name", name),
		zap.String("slot", privateKey.slot),
		zap.String("key_identity", ensure.Value(GetIdentityString(privateKey.key))),
	)

	ks := &keystorage.KeyStorage{}

	// We do not have a public key for our private key, so let's create one.
	armoredKey, err := crypto.NewKeyFromArmored(privateKey.key)
	if err != nil {
		return nil, err
	}

	publicKey, err := armoredKey.GetArmoredPublicKey()
	if err != nil {
		return nil, err
	}

	err = ks.InitializeRnd(rand.Reader, privateKey.slot, publicKey)
	if err != nil {
		return nil, err
	}

	if len(publicKeys) > 0 {
		// We have new public keys, let's try to add them to the key storage
		err = storePublicKeys(ks, privateKey, publicKeys, logger)
		if err != nil {
			return nil, err
		}
	}

	version := int64(0)

	err = recStore.Update(context.Background(), ks, version)
	if err != nil {
		return nil, err
	}

	version++

	return &KeyProvider{
		recstore:   recStore,
		privateKey: privateKey,
		version:    version,
	}, nil
}

// PublicKeyData is a public key and a slot.
type PublicKeyData struct {
	slot string
	key  string
}

// MakePublicKeyData creates a new public key.
func MakePublicKeyData(key string) (PublicKeyData, error) {
	if key == "" {
		return PublicKeyData{}, errors.New("key is empty")
	}

	identityString, err := GetIdentityString(key)
	if err != nil {
		return PublicKeyData{}, err
	}

	return PublicKeyData{slot: identityString, key: key}, nil
}

// PrivateKeyData is a private key and a slot.
type PrivateKeyData struct {
	slot string
	key  string
}

// MakePrivateKeyData creates a new private key.
func MakePrivateKeyData(key string) (PrivateKeyData, error) {
	if key == "" {
		return PrivateKeyData{}, errors.New("key is empty")
	}

	identityString, err := GetIdentityString(key)
	if err != nil {
		return PrivateKeyData{}, err
	}

	return PrivateKeyData{slot: identityString, key: key}, nil
}
