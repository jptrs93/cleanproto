
/**
 * @typedef {Object} Book
 * @property {string} id
 * @property {string} title
 * @property {string} author
 */
/**
 * @typedef {Object} Library
 * @property {string} id
 * @property {string} name
 * @property {Book[]} books
 */
/**
 * @typedef {Object} GetBookReq
 * @property {string} id
 */
/**
 * @typedef {Object} CheckoutBookReq
 * @property {string} libraryId
 * @property {string} bookId
 */
/**
 * @typedef {Object} ApiErr
 * @property {number} code
 * @property {string} displayErr
 * @property {string} internalErr
 */
import protobufjsm from 'protobufjs/minimal';
const { Reader, Writer } = protobufjsm;

const WIRE = {
    VARINT: 0,
    FIXED64: 1,
    LDELIM: 2,
    FIXED32: 5
};

const tag = (field, wire) => (field << 3) | wire;


/**
 * @param {Book} message
 * @param {Writer} writer
 */
export function writeBook(message, writer) {
    if (message.id !== undefined && message.id !== null && message.id !== "") {
        writer.uint32(tag(1, WIRE.LDELIM)).string(message.id);
    }
    if (message.title !== undefined && message.title !== null && message.title !== "") {
        writer.uint32(tag(2, WIRE.LDELIM)).string(message.title);
    }
    if (message.author !== undefined && message.author !== null && message.author !== "") {
        writer.uint32(tag(3, WIRE.LDELIM)).string(message.author);
    }
}


/**
 * @param {Book} message
 * @returns {Uint8Array}
 */
export function encodeBook(message) {
    const writer = Writer.create();
    writeBook(message, writer);
    return writer.finish();
}


/**
 * @param {Reader} reader
 * @param {number} [length]
 * @returns {Book}
 */
function decodeBookMessage(reader, length) {
    const end = length === undefined ? reader.len : reader.pos + length;
    const message = {id: "", title: "", author: "" };
    while (reader.pos < end) {
        const tag = reader.uint32();
        switch (tag >>> 3) {
            case 1: {
                message.id = reader.string();
                break;
            }
            case 2: {
                message.title = reader.string();
                break;
            }
            case 3: {
                message.author = reader.string();
                break;
            }
            default:
                reader.skipType(tag & 7);
        }
    }
    return message;
}


/**
 * @param {ArrayBuffer} buffer
 * @returns {Book}
 */
export function decodeBook(buffer) {
    const reader = Reader.create(new Uint8Array(buffer));
    return decodeBookMessage(reader);
}



/**
 * @param {Library} message
 * @param {Writer} writer
 */
export function writeLibrary(message, writer) {
    if (message.id !== undefined && message.id !== null && message.id !== "") {
        writer.uint32(tag(1, WIRE.LDELIM)).string(message.id);
    }
    if (message.name !== undefined && message.name !== null && message.name !== "") {
        writer.uint32(tag(2, WIRE.LDELIM)).string(message.name);
    }
    if (message.books && message.books.length > 0) {
        for (const item of message.books) {
            writer.uint32(tag(3, WIRE.LDELIM)).fork();
            writeBook(item, writer);
            writer.ldelim();
        }
    }
}


/**
 * @param {Library} message
 * @returns {Uint8Array}
 */
export function encodeLibrary(message) {
    const writer = Writer.create();
    writeLibrary(message, writer);
    return writer.finish();
}


/**
 * @param {Reader} reader
 * @param {number} [length]
 * @returns {Library}
 */
function decodeLibraryMessage(reader, length) {
    const end = length === undefined ? reader.len : reader.pos + length;
    const message = {id: "", name: "", books: [] };
    while (reader.pos < end) {
        const tag = reader.uint32();
        switch (tag >>> 3) {
            case 1: {
                message.id = reader.string();
                break;
            }
            case 2: {
                message.name = reader.string();
                break;
            }
            case 3: {
                message.books.push(decodeBookMessage(reader, reader.uint32()));
                break;
            }
            default:
                reader.skipType(tag & 7);
        }
    }
    return message;
}


/**
 * @param {ArrayBuffer} buffer
 * @returns {Library}
 */
export function decodeLibrary(buffer) {
    const reader = Reader.create(new Uint8Array(buffer));
    return decodeLibraryMessage(reader);
}



/**
 * @param {GetBookReq} message
 * @param {Writer} writer
 */
export function writeGetBookReq(message, writer) {
    if (message.id !== undefined && message.id !== null && message.id !== "") {
        writer.uint32(tag(1, WIRE.LDELIM)).string(message.id);
    }
}


/**
 * @param {GetBookReq} message
 * @returns {Uint8Array}
 */
export function encodeGetBookReq(message) {
    const writer = Writer.create();
    writeGetBookReq(message, writer);
    return writer.finish();
}


/**
 * @param {Reader} reader
 * @param {number} [length]
 * @returns {GetBookReq}
 */
function decodeGetBookReqMessage(reader, length) {
    const end = length === undefined ? reader.len : reader.pos + length;
    const message = {id: "" };
    while (reader.pos < end) {
        const tag = reader.uint32();
        switch (tag >>> 3) {
            case 1: {
                message.id = reader.string();
                break;
            }
            default:
                reader.skipType(tag & 7);
        }
    }
    return message;
}


/**
 * @param {ArrayBuffer} buffer
 * @returns {GetBookReq}
 */
export function decodeGetBookReq(buffer) {
    const reader = Reader.create(new Uint8Array(buffer));
    return decodeGetBookReqMessage(reader);
}



/**
 * @param {CheckoutBookReq} message
 * @param {Writer} writer
 */
export function writeCheckoutBookReq(message, writer) {
    if (message.libraryId !== undefined && message.libraryId !== null && message.libraryId !== "") {
        writer.uint32(tag(1, WIRE.LDELIM)).string(message.libraryId);
    }
    if (message.bookId !== undefined && message.bookId !== null && message.bookId !== "") {
        writer.uint32(tag(2, WIRE.LDELIM)).string(message.bookId);
    }
}


/**
 * @param {CheckoutBookReq} message
 * @returns {Uint8Array}
 */
export function encodeCheckoutBookReq(message) {
    const writer = Writer.create();
    writeCheckoutBookReq(message, writer);
    return writer.finish();
}


/**
 * @param {Reader} reader
 * @param {number} [length]
 * @returns {CheckoutBookReq}
 */
function decodeCheckoutBookReqMessage(reader, length) {
    const end = length === undefined ? reader.len : reader.pos + length;
    const message = {libraryId: "", bookId: "" };
    while (reader.pos < end) {
        const tag = reader.uint32();
        switch (tag >>> 3) {
            case 1: {
                message.libraryId = reader.string();
                break;
            }
            case 2: {
                message.bookId = reader.string();
                break;
            }
            default:
                reader.skipType(tag & 7);
        }
    }
    return message;
}


/**
 * @param {ArrayBuffer} buffer
 * @returns {CheckoutBookReq}
 */
export function decodeCheckoutBookReq(buffer) {
    const reader = Reader.create(new Uint8Array(buffer));
    return decodeCheckoutBookReqMessage(reader);
}



/**
 * @param {ApiErr} message
 * @param {Writer} writer
 */
export function writeApiErr(message, writer) {
    if (message.code !== undefined && message.code !== null && message.code !== 0) {
        writer.uint32(tag(1, WIRE.VARINT)).int32(message.code);
    }
    if (message.displayErr !== undefined && message.displayErr !== null && message.displayErr !== "") {
        writer.uint32(tag(2, WIRE.LDELIM)).string(message.displayErr);
    }
    if (message.internalErr !== undefined && message.internalErr !== null && message.internalErr !== "") {
        writer.uint32(tag(3, WIRE.LDELIM)).string(message.internalErr);
    }
}


/**
 * @param {ApiErr} message
 * @returns {Uint8Array}
 */
export function encodeApiErr(message) {
    const writer = Writer.create();
    writeApiErr(message, writer);
    return writer.finish();
}


/**
 * @param {Reader} reader
 * @param {number} [length]
 * @returns {ApiErr}
 */
function decodeApiErrMessage(reader, length) {
    const end = length === undefined ? reader.len : reader.pos + length;
    const message = {code: 0, displayErr: "", internalErr: "" };
    while (reader.pos < end) {
        const tag = reader.uint32();
        switch (tag >>> 3) {
            case 1: {
                message.code = reader.int32();
                break;
            }
            case 2: {
                message.displayErr = reader.string();
                break;
            }
            case 3: {
                message.internalErr = reader.string();
                break;
            }
            default:
                reader.skipType(tag & 7);
        }
    }
    return message;
}


/**
 * @param {ArrayBuffer} buffer
 * @returns {ApiErr}
 */
export function decodeApiErr(buffer) {
    const reader = Reader.create(new Uint8Array(buffer));
    return decodeApiErrMessage(reader);
}



