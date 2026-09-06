export type FieldType = 'string' | 'number' | 'boolean' | 'array' | 'object';
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
  /**
   * Id of a lookup table whose keys are this field's permitted values, exactly
   * as `OutputField.lookup` is. It declares the field's domain so a condition
   * offers the table's keys instead of a free-text box — a declaration, not
   * enforcement: nothing checks an incoming value at evaluation.
   */
  lookup?: string;
}

export type InputSchema = Record<string, SchemaField>;

export interface OutputField {
  type: FieldType;
  /** Id of a lookup table whose keys are this field's permitted values. */
  lookup?: string;
  /**
   * The caller's contract. Enforced twice by the engine: an error at snapshot
   * load if no authoring path supplies it, and a warning at evaluation if it
   * is absent anyway. Defaults to false so declaring a field never blocks a
   * save.
   */
  required?: boolean;
}

export type OutputSchema = Record<string, OutputField>;

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
  /**
   * This item's authored values for the segment's output schema, keyed by
   * field name. Only a reporting rule's outputs are read — never an inner
   * And/Or branch's.
   */
  outputs?: Record<string, string>;
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
  /** Values for output fields that do not vary per reported item. */
  outputs?: Record<string, string>;
}

export interface Layer {
  /** Stable identity: the response key, what dependsOn holds, what layer:x resolves. */
  key: string;
  /** Friendly label. Optional, free-form, references nothing. */
  name?: string;
  /**
   * Layers this one must follow. A rule referencing `layer:x` must declare x
   * here. If a dependency does not resolve, this layer is skipped rather than
   * evaluated against absent context.
   */
  dependsOn?: string[];
  segments: Segment[];
  /** Fallback locale for message rendering; empty means "en". */
  defaultLanguage?: string;
  inputSchema?: InputSchema;
  /** Declares the fields this layer's segments emit with each reported item. */
  outputSchema?: OutputSchema;
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
  /** The resolved output record for this finding. */
  outputs?: Record<string, unknown>;
}

export interface LayerResult {
  status: LayerStatus;
  segment?: string;
  strategy?: string;
  reason?: string;
  computed?: Record<string, unknown>;
  messages?: Record<string, string>;
  failures?: Failure[];
  /** The resolved output record, when a single-value strategy reported one. */
  outputs?: Record<string, unknown>;
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
  /**
   * Position in the table's ordering. Always persisted, even when inferred
   * from list position, because a relational store cannot reorder rows
   * cheaply.
   */
  order?: number;
}

export interface LookupTable {
  id: string;
  name: string;
  keyType: FieldType;
  /** The author's note on how the table is meant to be used, including any cross-table ordering scheme. */
  description?: string;
  /** Include each entry's `order` in the evaluation response. */
  emitOrder?: boolean;
  /** Numbers are hand-authored rather than inferred from list position. */
  customOrder?: boolean;
  entries: LookupEntry[];
}

/** One thing a search query matched. Mirrors model.SearchHit. */
export interface SearchHit {
  kind: 'layer' | 'segment';
  layer: string;
  segment?: string;
  field: 'key' | 'name' | 'id' | 'strategy';
  value: string;
}

export interface SearchResult {
  query: string;
  hits: SearchHit[];
  /** The store stopped at its limit and more matches exist. */
  truncated: boolean;
}
