import {
  decodeBook,
  decodeLibrary,
  encodeCheckoutBookReq,
  encodeGetBookReq,
} from './model';
import type {
  Book,
  CheckoutBookReq,
  GetBookReq,
  Library,
} from './model';

type HeaderProvider = () => Record<string, string>;
type ErrorHandler = (response: Response) => Promise<never>;
type RequestBody = BodyInit | Uint8Array<ArrayBufferLike>;

export class Capi {
  baseURL: string;
  headerProvider: HeaderProvider;

  errorHandler: ErrorHandler;

  constructor(baseURL = '', headerProvider: HeaderProvider | null = null, errorHandler: ErrorHandler | null = null) {
    this.baseURL = baseURL;
    this.headerProvider = headerProvider == null ? () => ({}) : headerProvider;
    this.errorHandler = errorHandler == null ? async (response: Response) => { throw new Error(`HTTP ${response.status}`); } : errorHandler;
  }

  async #request(path: string, { method = 'GET', body }: { method?: string; body?: RequestBody } = {}): Promise<Response> {
    const headers = this.headerProvider() || {};
    headers['Accept'] = 'application/x-protobuf';
    if (body !== undefined) {
      headers['Content-Type'] = 'application/x-protobuf';
    }
    return fetch(`${this.baseURL}${path}`, { method, headers, body, credentials: 'include' });
  }

  async getLibraryV1(): Promise<Library> {
    const response = await this.#request('/library/v1', { method: 'GET' });
    if (!response.ok) {
      return this.errorHandler(response);
    }
    return decodeLibrary(await response.arrayBuffer());
  }

  async getLibraryBookV1(payload: GetBookReq): Promise<Book> {
    const response = await this.#request('/library/book/v1', { method: 'GET', body: encodeGetBookReq(payload) });
    if (!response.ok) {
      return this.errorHandler(response);
    }
    return decodeBook(await response.arrayBuffer());
  }

  async postLibraryBook_CheckoutV1(payload: CheckoutBookReq): Promise<void> {
    const response = await this.#request('/library/book-checkout-v1', { method: 'POST', body: encodeCheckoutBookReq(payload) });
    if (!response.ok) {
      return this.errorHandler(response);
    }
    await response.arrayBuffer();
  }

}
