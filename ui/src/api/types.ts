export type FieldType = 'string' | 'number' | 'boolean' | 'array';
export type Operator = 'eq' | 'neq' | 'gt' | 'gte' | 'lt' | 'lte' | 'in' | 'contains' | 'in_lookup' | 'not_in_lookup';
export type CompositeOperator = 'And' | 'Or';
export type StrategyType = 'static' | 'rule' | 'percentage' | 'expression';

export interface SchemaField {
  type: FieldType;
  required: boolean;
}

export type InputSchema = Record<string, SchemaField>;

export interface Expression {
  field: string;
  operator: Operator;
  value: unknown;
}

export interface Rule {
  ruleName: string;
  operator?: CompositeOperator;
  enabled?: boolean;
  successEvent?: string;
  errorMessage?: string;
  expression?: Expression;
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

export interface ExpressionDef {
  name: string;
  type: FieldType;
  expression: string;
}

export interface Segment {
  id: string;
  strategy: StrategyType;
  static?: StaticConfig;
  percentage?: PercentageConfig;
  expressions?: ExpressionDef[];
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
  order: number;
  segments: Segment[];
  /** Fallback locale for message rendering; empty means "en". */
  defaultLanguage?: string;
}

export interface Snapshot {
  version: number;
  layers: Layer[];
  lookups?: LookupTable[];
}

export interface LayerResult {
  segment: string;
  strategy: string;
  reason: string;
  expressions?: Record<string, unknown>;
  messages?: Record<string, string>;
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
  contains: ['array', 'string'],
  in_lookup: ['string', 'number'],
  not_in_lookup: ['string', 'number'],
};

/** Operators whose value references a lookup table id. */
export const LOOKUP_OPERATORS: Operator[] = ['in_lookup', 'not_in_lookup'];

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
