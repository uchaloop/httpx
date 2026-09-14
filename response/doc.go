/*
Package response describes a successful HTTP answer as a value, before anything
is written.

A handler returns a [Response]: the status, the headers and a JSON body. The
value knows nothing about the router, so the same answer can be written by
net/http with [Write] or by a router with its own serializer.

# Constructors

[JSON] answers any value with a status. [Data] wraps the value in a data
envelope; [OK] and [Created] are its 200 and 201 forms, and Created sets the
Location header:

	response.Created("/articles/42", article)
	// 201, Location: /articles/42
	// {"data":{"id":42,"title":"HTTP boundaries"}}

[NoContent] answers 204 without a body. [Page] answers one page of items with
the total and the page position:

	{"data":{"items":[...],"total":42,"page":2,"size":20}}

Nil items are written as an empty array, so a client never has to tell null
from an empty list.

# Headers

Location and Content-Type have fields of their own; any other header, such as
a custom X-User, goes to Header. [Response.SetHeaders] writes them all under
canonical names, replacing values already set, so x-user and X-User are the
same header whichever way a handler spells it.

# Writing

[Response.Validate] rejects a response that must not be written: a status
outside 200-599, a body on 204 or 304, a body without a content type, or 201
without a Location.

[Write] is the net/http backend. It validates the response and encodes the body
with encoding/json before it commits anything, so after a failure the writer is
untouched and the handler can still answer with an error.

A router that has its own serializer writes a Response the same way: Validate,
SetHeaders into the router's response headers, then Body with Status, or only
Status when ContentType is empty.
*/
package response
