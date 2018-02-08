const chai = require("chai");
const expect = chai.expect;

const misc = require("../../lib/misc");

describe("misc hex utilities", () => {

    const text = "43ef12ac1bc8";
    it("hextoUint8Array returns a Uint8Array buffer", () => {
        const buffer = misc.hexToUint8Array(text);
        expect(buffer).to.be.a("Uint8Array");
    });

    it("hex correctly decodes from a Uint8Array buffer", () => {
        const buffer = misc.hexToUint8Array(text)
        expect(buffer).to.be.a("Uint8Array");
        const expected = misc.uint8ArrayToHex(buffer);
        expect(expected).to.be.a("string");
        expect(expected).to.have.lengthOf(text.length);
        expect(expected).to.be.deep.equal(text);
    });
});

describe("misc buffer equality", () => {

    it("returns true for equal buffers", () => {
        const buffer1 = new Uint8Array([1,2,3,4]);
        const buffer2 = new Uint8Array([1,2,3,4]);
        expect(misc.uint8ArrayCompare(buffer1,buffer2)).to.be.true;
    });

    it("returns false for different buffers", () => {
        const buffer1 = new Uint8Array([1,2,3,4]);
        const buffer2 = new Uint8Array([1,2,3,3]);
        expect(misc.uint8ArrayCompare(buffer1,buffer2)).to.be.false;
    });

});

describe("misc bitmask", () => {

    it("returns the right bit set", () => {

        // try with 6 bit set amongst 16 bits
        // "0110 1101 0000 0010";
        const buffer = new Buffer(2);
        buffer.writeUInt8(0x6d,0);
        buffer.writeUInt8(0x02,1);
        const bitmask = Uint8Array.from(buffer);
        const nb = misc.getBitmaskLength(bitmask)
        expect(nb).to.be.equal(16);
        console.log(convert(bitmask).toString(2));
        const indices = misc.getSetBits(bitmask);
        const expected = [1,2,4,5,7,14];
        console.log(indices);
        console.log(expected);
        expect(indices).to.be.deep.equal(expected);

    });

});

function convert(Uint8Arr) {
    var length = Uint8Arr.length;

    let buffer = Buffer.from(Uint8Arr);
    var result = buffer.readUIntBE(0, length);

    return result;
}