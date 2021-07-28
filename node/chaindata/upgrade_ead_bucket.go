/*
 * Copyright (c) 2021 The JaxNetwork developers
 * Use of this source code is governed by an ISC
 * license that can be found in the LICENSE file.
 */

package chaindata

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"io"

	"gitlab.com/jaxnet/jaxnetd/database"
	"gitlab.com/jaxnet/jaxnetd/node/encoder"
	"gitlab.com/jaxnet/jaxnetd/types/chainhash"
	"gitlab.com/jaxnet/jaxnetd/types/wire"
)

func MigrateEADAddresses(dbTx database.Tx) error {
	if !dbTx.Chain().IsBeacon() {
		return nil
	}

	utxoBucketV1 := dbTx.Metadata().Bucket(EADAddressesBucketNameV1)
	if utxoBucketV1 == nil {
		return nil
	}

	log.Info().Msg("Migration EADAddresses bucket from v1 to v2. This might take a while...")

	view := map[string]wire.EADAddresses{}
	err := utxoBucketV1.ForEach(func(rawKey, serializedData []byte) error {
		if rawKey == nil || serializedData == nil {
			return nil
		}

		r := bytes.NewBuffer(serializedData)
		eadAddress, err := DecodeEADAddressesV1(r, wire.ProtocolVersion)
		if err != nil {
			return database.Error{
				ErrorCode:   database.ErrCorruption,
				Description: fmt.Sprintf("corrupt ead addresses entry for %v: %v", string(rawKey), err),
			}
		}

		view[string(rawKey)] = eadAddress
		return nil
	})

	utxoBucketV2 := dbTx.Metadata().Bucket(EADAddressesBucketNameV2)
	if utxoBucketV2 == nil {
		var err error
		utxoBucketV2, err = dbTx.Metadata().CreateBucket(EADAddressesBucketNameV2)
		if err != nil {
			return err
		}
		return nil
	}
	for s, addresses := range view {
		w := bytes.NewBuffer(nil)
		err = addresses.BtcEncode(w, wire.ProtocolVersion, wire.BaseEncoding)
		if err != nil {
			return err
		}

		err = utxoBucketV2.Put([]byte(s), w.Bytes())
		if err != nil {
			return err
		}
	}

	log.Info().Int("count", len(view)).Msg("Migration of EADAddresses bucket from v1 to v2 finished.")

	return dbTx.Metadata().DeleteBucket(EADAddressesBucketNameV1)
}

func DecodeEADAddressesV1(r io.Reader, pver uint32) (wire.EADAddresses, error) {
	var msg wire.EADAddresses
	alias, err := encoder.ReadVarInt(r, pver)
	if err != nil {
		return msg, err
	}
	msg.ID = alias

	msg.OwnerPubKey, err = encoder.ReadVarBytes(r, pver, 65*2, "OwnerPubKey")
	if err != nil {
		return msg, err
	}
	count, err := encoder.ReadVarInt(r, pver)
	if err != nil {
		return msg, err
	}
	msg.Addresses = make([]wire.EADAddress, count)

	for i := range msg.Addresses {
		msg.Addresses[i], err = DecodeEADAddressV1(r, pver)
		if err != nil {
			return msg, err
		}
	}

	return msg, nil
}

func DecodeEADAddressV1(r io.Reader, pver uint32) (wire.EADAddress, error) {
	var msg wire.EADAddress
	var ip [16]byte

	err := encoder.ReadElements(r, (*encoder.Uint32Time)(&msg.ExpiresAt), &ip)
	if err != nil {
		return msg, err
	}
	msg.IP = ip[:]

	id, err := encoder.ReadVarInt(r, pver)
	if err != nil {
		return msg, err
	}
	msg.Shard = uint32(id)

	// Sigh. Bitcoin protocol mixes little and big endian.
	port, err := encoder.BinarySerializer.Uint16(r, binary.BigEndian)
	if err != nil {
		return msg, err
	}
	msg.Port = port

	rawHash, err := encoder.ReadVarBytes(r, pver, chainhash.HashSize, "TxHash")
	if err != nil {
		return msg, err
	}

	msg.TxHash, err = chainhash.NewHash(rawHash)
	if err != nil {
		return msg, err
	}

	txOutId, err := encoder.ReadVarInt(r, pver)
	if err != nil {
		return msg, err
	}
	msg.TxOutIndex = int(txOutId)

	return msg, nil
}
