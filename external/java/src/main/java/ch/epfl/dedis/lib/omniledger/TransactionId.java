package ch.epfl.dedis.lib.omniledger;

import ch.epfl.dedis.lib.HashId;
import ch.epfl.dedis.lib.crypto.Hex;
import ch.epfl.dedis.lib.exception.CothorityCryptoException;
import ch.epfl.dedis.lib.omniledger.darc.DarcId;
import com.google.protobuf.ByteString;

import java.util.Arrays;

/**
 * Implementation of {@link HashId}. This implementation is immutable and is can be used as key for collections
 */
public class TransactionId implements HashId {
    private final byte[] id;
    public final static int length = 64;

    public TransactionId(byte[] id) throws CothorityCryptoException {
        if (id.length != length) {
            throw new CothorityCryptoException("need 64 bytes for instanceID, only got " + id.length);
        }
        this.id = Arrays.copyOf(id, id.length);
    }

    public TransactionId(DarcId did, SubId sid){
        id = new byte[length];
        System.arraycopy(did.getId(), 0, id, 0, 32);
        System.arraycopy(sid.getId(), 0, id, 32, 32);
    }

    @Override
    public byte[] getId() {
        return Arrays.copyOf(id, id.length);
    }

    /**
     * @return the baseId of the darc responsible for this instance
     * @throws CothorityCryptoException
     */
    public DarcId getDarcId() throws CothorityCryptoException {
        return new DarcId(Arrays.copyOf(id, 32));
    }

    /**
     * @return the subId of the instance in the responsible darc-namespace
     * @throws CothorityCryptoException
     */
    public SubId getSubId() throws CothorityCryptoException{
        return new SubId(Arrays.copyOfRange(id, 32, 64));
    }

    @Override
    public boolean equals(Object o) {
        if (this == o) return true;
        if (o == null || getClass() != o.getClass()) return false;

        return Arrays.equals(id, ((TransactionId) o).id);
    }

    @Override
    public int hashCode() {
        return Arrays.hashCode(id);
    }

    @Override
    public String toString(){
        return Hex.printHexBinary(id);
    }

    public ByteString toProto(){
        return ByteString.copyFrom(id);
    }
}
