package example

import (
	"google.golang.org/protobuf/encoding/protowire"
)

type Book struct {
	ID     string
	Title  string
	Author string
}

func (m *Book) Encode() []byte {
	var b []byte
	b = AppendStringField(b, m.ID, 1)
	b = AppendStringField(b, m.Title, 2)
	b = AppendStringField(b, m.Author, 3)
	return b
}

func DecodeBook(b []byte) (*Book, error) {
	var m Book
	var num protowire.Number
	var typ protowire.Type
	var err error
	for len(b) > 0 {
		b, num, typ, err = ConsumeTag(b)
		if err != nil {
			return nil, err
		}
		switch num {
		case 1:
			b, m.ID, err = ConsumeString(b, typ)
		case 2:
			b, m.Title, err = ConsumeString(b, typ)
		case 3:
			b, m.Author, err = ConsumeString(b, typ)
		default:
			b, err = SkipFieldValue(b, num, typ)
		}
		if err != nil {
			return nil, err
		}
	}
	return &m, nil
}

type Library struct {
	ID    string
	Name  string
	Books []*Book
}

func (m *Library) Encode() []byte {
	var b []byte
	b = AppendStringField(b, m.ID, 1)
	b = AppendStringField(b, m.Name, 2)
	for _, item := range m.Books {
		if item == nil {
			continue
		}
		b = protowire.AppendTag(b, 3, protowire.BytesType)
		b = protowire.AppendBytes(b, item.Encode())
	}
	return b
}

func DecodeLibrary(b []byte) (*Library, error) {
	var m Library
	var num protowire.Number
	var typ protowire.Type
	var err error
	var msgBytes []byte
	for len(b) > 0 {
		b, num, typ, err = ConsumeTag(b)
		if err != nil {
			return nil, err
		}
		switch num {
		case 1:
			b, m.ID, err = ConsumeString(b, typ)
		case 2:
			b, m.Name, err = ConsumeString(b, typ)
		case 3:
			b, msgBytes, err = ConsumeMessage(b, typ)
			if err == nil {
				var item *Book
				item, err = DecodeBook(msgBytes)
				if err == nil {
					m.Books = append(m.Books, item)
				}
			}
		default:
			b, err = SkipFieldValue(b, num, typ)
		}
		if err != nil {
			return nil, err
		}
	}
	return &m, nil
}

type GetBookReq struct {
	ID string
}

func (m *GetBookReq) Encode() []byte {
	var b []byte
	b = AppendStringField(b, m.ID, 1)
	return b
}

func DecodeGetBookReq(b []byte) (*GetBookReq, error) {
	var m GetBookReq
	var num protowire.Number
	var typ protowire.Type
	var err error
	for len(b) > 0 {
		b, num, typ, err = ConsumeTag(b)
		if err != nil {
			return nil, err
		}
		switch num {
		case 1:
			b, m.ID, err = ConsumeString(b, typ)
		default:
			b, err = SkipFieldValue(b, num, typ)
		}
		if err != nil {
			return nil, err
		}
	}
	return &m, nil
}

type CheckoutBookReq struct {
	LibraryID string
	BookID    string
}

func (m *CheckoutBookReq) Encode() []byte {
	var b []byte
	b = AppendStringField(b, m.LibraryID, 1)
	b = AppendStringField(b, m.BookID, 2)
	return b
}

func DecodeCheckoutBookReq(b []byte) (*CheckoutBookReq, error) {
	var m CheckoutBookReq
	var num protowire.Number
	var typ protowire.Type
	var err error
	for len(b) > 0 {
		b, num, typ, err = ConsumeTag(b)
		if err != nil {
			return nil, err
		}
		switch num {
		case 1:
			b, m.LibraryID, err = ConsumeString(b, typ)
		case 2:
			b, m.BookID, err = ConsumeString(b, typ)
		default:
			b, err = SkipFieldValue(b, num, typ)
		}
		if err != nil {
			return nil, err
		}
	}
	return &m, nil
}

type ApiErr struct {
	Code        int32
	DisplayErr  string
	InternalErr string
}

func (m *ApiErr) Encode() []byte {
	var b []byte
	b = AppendInt32Field(b, m.Code, 1)
	b = AppendStringField(b, m.DisplayErr, 2)
	return b
}

func DecodeApiErr(b []byte) (*ApiErr, error) {
	var m ApiErr
	var num protowire.Number
	var typ protowire.Type
	var err error
	for len(b) > 0 {
		b, num, typ, err = ConsumeTag(b)
		if err != nil {
			return nil, err
		}
		switch num {
		case 1:
			b, m.Code, err = ConsumeVarInt32(b, typ)
		case 2:
			b, m.DisplayErr, err = ConsumeString(b, typ)
		case 3:
			b, m.InternalErr, err = ConsumeString(b, typ)
		default:
			b, err = SkipFieldValue(b, num, typ)
		}
		if err != nil {
			return nil, err
		}
	}
	return &m, nil
}
