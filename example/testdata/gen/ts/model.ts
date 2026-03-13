import protobufjsm from 'protobufjs/minimal';
import type { Reader, Writer } from 'protobufjs/minimal';
const { Reader, Writer } = protobufjsm;
export interface Book {
  id: string;
  title: string;
  author: string;
}
export interface Library {
  id: string;
  name: string;
  books: Book[];
}
export interface GetBookReq {
  id: string;
}
export interface CheckoutBookReq {
  libraryId: string;
  bookId: string;
}
export interface ApiErr {
  code: number;
  displayErr: string;
  internalErr: string;
}

const WIRE = {
    VARINT: 0,
    FIXED64: 1,
    LDELIM: 2,
    FIXED32: 5
};

const tag = (field: number, wire: number): number => (field << 3) | wire;


export function writeBook(message: Book, writer: Writer): void {
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


export function encodeBook(message: Book): Uint8Array {
    const writer = Writer.create();
    writeBook(message, writer);
    return writer.finish();
}


function decodeBookMessage(reader: Reader, length?: number): Book {
    const end = length === undefined ? reader.len : reader.pos + length;
    const message: Book = {id: "", title: "", author: "" };
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


export function decodeBook(buffer: ArrayBuffer): Book {
    const reader = Reader.create(new Uint8Array(buffer));
    return decodeBookMessage(reader);
}



export function writeLibrary(message: Library, writer: Writer): void {
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


export function encodeLibrary(message: Library): Uint8Array {
    const writer = Writer.create();
    writeLibrary(message, writer);
    return writer.finish();
}


function decodeLibraryMessage(reader: Reader, length?: number): Library {
    const end = length === undefined ? reader.len : reader.pos + length;
    const message: Library = {id: "", name: "", books: [] };
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


export function decodeLibrary(buffer: ArrayBuffer): Library {
    const reader = Reader.create(new Uint8Array(buffer));
    return decodeLibraryMessage(reader);
}



export function writeGetBookReq(message: GetBookReq, writer: Writer): void {
    if (message.id !== undefined && message.id !== null && message.id !== "") {
        writer.uint32(tag(1, WIRE.LDELIM)).string(message.id);
    }
}


export function encodeGetBookReq(message: GetBookReq): Uint8Array {
    const writer = Writer.create();
    writeGetBookReq(message, writer);
    return writer.finish();
}


function decodeGetBookReqMessage(reader: Reader, length?: number): GetBookReq {
    const end = length === undefined ? reader.len : reader.pos + length;
    const message: GetBookReq = {id: "" };
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


export function decodeGetBookReq(buffer: ArrayBuffer): GetBookReq {
    const reader = Reader.create(new Uint8Array(buffer));
    return decodeGetBookReqMessage(reader);
}



export function writeCheckoutBookReq(message: CheckoutBookReq, writer: Writer): void {
    if (message.libraryId !== undefined && message.libraryId !== null && message.libraryId !== "") {
        writer.uint32(tag(1, WIRE.LDELIM)).string(message.libraryId);
    }
    if (message.bookId !== undefined && message.bookId !== null && message.bookId !== "") {
        writer.uint32(tag(2, WIRE.LDELIM)).string(message.bookId);
    }
}


export function encodeCheckoutBookReq(message: CheckoutBookReq): Uint8Array {
    const writer = Writer.create();
    writeCheckoutBookReq(message, writer);
    return writer.finish();
}


function decodeCheckoutBookReqMessage(reader: Reader, length?: number): CheckoutBookReq {
    const end = length === undefined ? reader.len : reader.pos + length;
    const message: CheckoutBookReq = {libraryId: "", bookId: "" };
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


export function decodeCheckoutBookReq(buffer: ArrayBuffer): CheckoutBookReq {
    const reader = Reader.create(new Uint8Array(buffer));
    return decodeCheckoutBookReqMessage(reader);
}



export function writeApiErr(message: ApiErr, writer: Writer): void {
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


export function encodeApiErr(message: ApiErr): Uint8Array {
    const writer = Writer.create();
    writeApiErr(message, writer);
    return writer.finish();
}


function decodeApiErrMessage(reader: Reader, length?: number): ApiErr {
    const end = length === undefined ? reader.len : reader.pos + length;
    const message: ApiErr = {code: 0, displayErr: "", internalErr: "" };
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


export function decodeApiErr(buffer: ArrayBuffer): ApiErr {
    const reader = Reader.create(new Uint8Array(buffer));
    return decodeApiErrMessage(reader);
}



