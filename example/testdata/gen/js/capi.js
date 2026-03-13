import {
  decodeBook,
  decodeLibrary,
  encodeCheckoutBookReq,
  encodeGetBookReq,
} from './model.js';

/** @typedef {() => Object.<string, string>} HeaderProvider */
/** @typedef {(response: Response) => Promise<never>} ErrorHandler */
/** @typedef {BodyInit|Uint8Array} RequestBody */

export class Capi {
  /**
   * @param {string} [baseURL='']
   * @param {HeaderProvider | null} [headerProvider=null]
   * @param {ErrorHandler | null} [errorHandler=null]
   */
  constructor(baseURL = '', headerProvider = null, errorHandler = null) {
    this.baseURL = baseURL;
    this.headerProvider = headerProvider == null ? () => ({}) : headerProvider;
    this.errorHandler = errorHandler == null ? async (response) => { throw new Error(`HTTP ${response.status}`); } : errorHandler;
  }

  /**
   * @param {string} path
   * @param {{ method?: string, body?: RequestBody }} [options={}]
   * @returns {Promise<Response>}
   */
  async #request(path, { method = 'GET', body } = {}) {
    const headers = this.headerProvider() || {};
    headers['Accept'] = 'application/x-protobuf';
    if (body !== undefined) {
      headers['Content-Type'] = 'application/x-protobuf';
    }
    return fetch(`${this.baseURL}${path}`, { method, headers, body, credentials: 'include' });
  }

  /**
   * @returns {Promise<Library>}
   */
  async getLibraryV1() {
    const response = await this.#request('/library/v1', { method: 'GET' });
    if (!response.ok) {
      return this.errorHandler(response);
    }
    return decodeLibrary(await response.arrayBuffer());
  }

  /**
   * @param {GetBookReq} payload
   * @returns {Promise<Book>}
   */
  async getLibraryBookV1(payload) {
    const response = await this.#request('/library/book/v1', { method: 'GET', body: encodeGetBookReq(payload) });
    if (!response.ok) {
      return this.errorHandler(response);
    }
    return decodeBook(await response.arrayBuffer());
  }

  /**
   * @param {CheckoutBookReq} payload
   * @returns {Promise<void>}
   */
  async postLibraryBook_CheckoutV1(payload) {
    const response = await this.#request('/library/book-checkout-v1', { method: 'POST', body: encodeCheckoutBookReq(payload) });
    if (!response.ok) {
      return this.errorHandler(response);
    }
    await response.arrayBuffer();
  }

}
