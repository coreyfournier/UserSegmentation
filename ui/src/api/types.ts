export type FieldType = 'string' | 'number' | 'boolean' | 'array';
export type Operator =
  | 'eq'
  | 'neq'
  | 'gt'
  | 'gte'
  | 'lt'
  | 'lte'
  | 'in'
  | 'contains'
  | 'in_lookup'
  | 'not_in_lookup'
  | 'not_in'
  | 'is_null'
  | 'is_null_or_empty';
export type CompositeOperator = 'And' | 'Or';
export type StrategyType = 'static' | 'rule' | 'percentage' | 'checklist';

/**
 * Checklist layers report satisfied/violated/unevaluable; every other strategy
 * reports the neutral resolution vocabulary. Status is always authoritative —
 * never infer the outcome from `failures.length`.
 */
export type LayerStatus =
  | 'satisfied'
  | 'violated'
  | 'unevaluable'
  | 'resolved'
  | 'unresolved'
  | 'skipped';

export interface SchemaField {
  type: FieldType;
  required: boolean;
}

export type InputSchema = Record<string, SchemaField>;

export interface Condition {
  field: string;
  operator: Operator;
  /** Absent for unary operators, which test the field itself. */
  value?: unknown;
}

export interface Rule {
  ruleName: string;
  operator?: CompositeOperator;
  enabled?: boolean;
  successEvent?: string;
  errorMessage?: string;
  condition?: Condition;
  rules?: Rule[];
  /** Optional localized message templates keyed by language code (e.g. "en"). */
  messages?: Record<string, string>;
}

export interface Promotion {
  effective_from?: string;
  effective_until?: string;
}

export interface PercentageBucket {
  segment: string;
  weight: number;
}

export interface PercentageConfig {
  salt: string;
  buckets: PercentageBucket[];
}

export interface StaticConfig {
  mappings: Record<string, string>;
  default: string;
}

export interface ComputedField {
  name: string;
  type: FieldType;
  formula: string;
}

export interface Segment {
  id: string;
  /**
   * Dispatch predicate. When present and false the segment is passed over
   * entirely and produces no output — this is how one layer holds
   * per-entity-type variants.
   */
  when?: Rule;
  strategy: StrategyType;
  static?: StaticConfig;
  percentage?: PercentageConfig;
  computed?: ComputedField[];
  rules?: Rule[];
  overrides?: Rule[];
  default?: string;
  /** Localized messages rendered when the segment falls back to `default`. */
  defaultMessages?: Record<string, string>;
  promotion?: Promotion;
  inputSchema?: InputSchema;
}

export interface Layer {
  name: string;
  /**
   * Layers this one must follow. A rule referencing `layer:x` must declare x
   * here. If a dependency does not resolve, this layer is skipped rather than
   * evaluated against absent context.
   */
  dependsOn?: string[];
  segments: Segment[];
  /** Fallback locale for message rendering; empty means "en". */
  defaultLanguage?: string;
}

export interface Snapshot {
  version: number;
  layers: Layer[];
  lookups?: LookupTable[];
}

/** One itemised problem from a checklist layer. */
export interface Failure {
  /** Stable identifier — the rule name doubles as the public contract. */
  rule: string;
  message?: string;
  messages?: Record<string, string>;
}

export interface LayerResult {
  status: LayerStatus;
  segment?: string;
  strategy?: string;
  reason?: string;
  computed?: Record<string, unknown>;
  messages?: Record<string, string>;
  failures?: Failure[];
}

export interface Warning {
  segment: string;
  field: string;
  message: string;
}

export interface EvaluateRequest {
  subject_key: string;
  context: Record<string, unknown>;
  layers?: string[];
  languages?: string[];
  render_all?: boolean;
}

export interface EvaluateResponse {
  subject_key: string;
  layers: Record<string, LayerResult>;
  warnings?: Warning[];
  evaluated_at: string;
  duration_us: number;
}

export const OPERATOR_TYPES: Record<Operator, FieldType[]> = {
  eq: ['string', 'number', 'boolean'],
  neq: ['string', 'number', 'boolean'],
  gt: ['number'],
  gte: ['number'],
  lt: ['number'],
  lte: ['number'],
  in: ['string', 'number'],
  not_in: ['string', 'number'],
  contains: ['array', 'string'],
  in_lookup: ['string', 'number'],
  not_in_lookup: ['string', 'number'],
  // Any optional field of any type can be null.
  is_null: ['string', 'number', 'boolean', 'array'],
  // Emptiness here means the empty string, so this is a string test.
  is_null_or_empty: ['string'],
};

/** Operators whose value references a lookup table id. */
export const LOOKUP_OPERATORS: Operator[] = ['in_lookup', 'not_in_lookup'];

/**
 * Operators that test the field itself and take no value. They are also the
 * only ones that hold when the field is absent — absent is null.
 */
export const UNARY_OPERATORS: Operator[] = ['is_null', 'is_null_or_empty'];

export interface LookupEntry {
  key: unknown;
  value?: string;
}

export interface LookupTable {
  id: string;
  name: string;
  keyType: FieldType;
  entries: LookupEntry[];
}
