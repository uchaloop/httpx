/*
Package sortby parses client sort expressions into a typed order.

A client sends one expression per value, a field optionally followed by a
direction:

	GET /articles?sort=status&sort=updatedAt:desc

A field without a direction sorts in ascending order. [Parse] checks each
expression and converts its field with a function the application supplies, so
only the fields an endpoint allows reach the storage:

	order, err := sortby.Parse(query["sort"], func(name string) (Column, error) {
		switch name {
		case "status":
			return ColumnStatus, nil
		case "updatedAt":
			return ColumnUpdatedAt, nil
		default:
			return "", fmt.Errorf("unknown sort field %q", name)
		}
	})

The result is an [Order], the [Term] values in the order the client sent them.
[Order.Make] converts it into the application's own type, for example the sort
type of a storage package.

# Errors

Parse stops at the first invalid expression and returns a [*ParseError] with an
[ErrorKind] and the Index of the expression. The error of the field function is
available through errors.Unwrap. [ParseExpression] parses one expression
without restricting its field.
*/
package sortby
