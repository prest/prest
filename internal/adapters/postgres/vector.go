package postgres

import (
	"fmt"
	"math"
	"strconv"
	"strings"

	"github.com/prest/prest/v2/internal/ident"
)

// maxVectorDims mirrors pgvector's hard limit for the `vector` type (16000
// dimensions). Rejecting oversized inputs bounds the work done per request.
const maxVectorDims = 16000

// GetVectorOperator maps a pgvector distance metric name to its SQL operator.
// The result is a fixed constant from this whitelist — never derived from user
// bytes — so it is safe to interpolate directly into SQL.
func GetVectorOperator(metric string) (string, error) {
	switch strings.ToLower(strings.TrimSpace(metric)) {
	case "l2", "euclidean":
		return "<->", nil
	case "cosine", "cos":
		return "<=>", nil
	case "ip", "inner", "dot":
		return "<#>", nil
	case "l1", "manhattan":
		return "<+>", nil
	}
	return "", ErrInvalidVectorMetric
}

// normalizeVectorLiteral validates a pgvector literal like "[1,2,3]" and
// rebuilds it exclusively from parsed float64 values. Because every element is
// round-tripped through strconv.ParseFloat/FormatFloat, the returned string can
// only ever contain the byte set produced by FormatFloat ('g' format: digits,
// '.', '-', '+', 'e', plus the brackets and commas added here). No caller-
// supplied byte survives, which makes the literal safe to embed in SQL even
// where a bound parameter is not available (e.g. ORDER BY).
func normalizeVectorLiteral(s string) (string, error) {
	s = strings.TrimSpace(s)
	if len(s) < 2 || s[0] != '[' || s[len(s)-1] != ']' {
		return "", ErrInvalidVector
	}
	inner := strings.TrimSpace(s[1 : len(s)-1])
	if inner == "" {
		return "", ErrInvalidVector
	}
	parts := strings.Split(inner, ",")
	if len(parts) > maxVectorDims {
		return "", ErrInvalidVector
	}
	nums := make([]string, len(parts))
	for i, p := range parts {
		f, err := strconv.ParseFloat(strings.TrimSpace(p), 64)
		if err != nil || math.IsNaN(f) || math.IsInf(f, 0) {
			return "", ErrInvalidVector
		}
		nums[i] = strconv.FormatFloat(f, 'g', -1, 64)
	}
	return "[" + strings.Join(nums, ",") + "]", nil
}

// buildVectorOrderTerm parses a "<column>:<metric>:<vector>" spec (from the
// _korder query parameter) into a single ORDER BY term such as
// `"embedding" <-> '[1,2,3]'::vector`. Nearest-first ordering is the pgvector
// KNN default for every supported metric, so no direction is emitted.
//
// Safety: the column is validated and quoted via the ident package, the
// operator comes from GetVectorOperator's whitelist, and the vector literal is
// reconstructed by normalizeVectorLiteral. The single-quoted literal therefore
// cannot break out of its string context.
func buildVectorOrderTerm(spec string) (string, error) {
	parts := strings.SplitN(spec, ":", 3)
	if len(parts) != 3 {
		return "", ErrInvalidVectorOrder
	}
	col, metric, vec := parts[0], parts[1], parts[2]

	if !ident.IsValid(col) {
		return "", ErrInvalidIdentifier
	}
	op, err := GetVectorOperator(metric)
	if err != nil {
		return "", err
	}
	lit, err := normalizeVectorLiteral(vec)
	if err != nil {
		return "", err
	}
	q, _ := ident.Quote(col)
	return fmt.Sprintf(`%s %s '%s'::vector`, q, op, lit), nil
}

// buildVectorFilter parses a distance-threshold predicate for the :vecdist key
// suffix. The value format is "<metric>:<comparison>:<vector>:<threshold>", e.g.
// "l2:lt:[1,2,3]:0.5", producing `("col" <-> $n::vector) < $n+1`.
//
// Safety: the column is validated/quoted, the distance operator comes from the
// metric whitelist, and the comparison operator is restricted to scalar
// comparisons. Both the vector (normalized) and the threshold (parsed float)
// are passed as bound parameters, so no user value is interpolated into SQL.
func buildVectorFilter(column, value string, pid *int) (key string, values []interface{}, err error) {
	if !ident.IsValid(column) {
		return "", nil, ErrInvalidIdentifier
	}
	parts := strings.SplitN(value, ":", 4)
	if len(parts) != 4 {
		return "", nil, ErrInvalidVectorFilter
	}
	metric, cmp, vec, thr := parts[0], parts[1], parts[2], parts[3]

	distOp, err := GetVectorOperator(metric)
	if err != nil {
		return "", nil, err
	}
	cmpOp, err := GetQueryOperator(cmp)
	if err != nil {
		return "", nil, err
	}
	// Restrict to scalar comparisons; distance is a float, so IN/LIKE/IS NULL
	// and friends must not be accepted here.
	switch cmpOp {
	case "=", "!=", ">", ">=", "<", "<=":
	default:
		return "", nil, ErrInvalidOperator
	}
	lit, err := normalizeVectorLiteral(vec)
	if err != nil {
		return "", nil, err
	}
	threshold, perr := strconv.ParseFloat(strings.TrimSpace(thr), 64)
	if perr != nil || math.IsNaN(threshold) || math.IsInf(threshold, 0) {
		return "", nil, ErrInvalidVectorThreshold
	}

	q, _ := ident.Quote(column)
	key = fmt.Sprintf(`(%s %s $%d::vector) %s $%d`, q, distOp, *pid, cmpOp, *pid+1)
	values = []interface{}{lit, threshold}
	*pid += 2
	return key, values, nil
}
