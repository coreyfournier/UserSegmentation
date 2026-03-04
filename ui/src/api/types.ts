export type FieldType = 'string' | 'number' | 'boolean' | 'array';
export type Operator = 'eq' | 'neq' | 'gt' | 'gte' | 'lt' | 'lte' | 'in' | 'contains';
export type CompositeOperator = 'And' | 'Or';
export type StrategyType = 'static' | 'rule' | 'percentage';

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

export interface Segment {
  id: string;
  strategy: StrategyType;
  static?: StaticConfig;
  percentage?: PercentageConfig;
  rules?: Rule[];
  overrides?: Rule[];
  default?: string;
  promotion?: Promotion;
  inputSchema?: InputSchema;
}

export interface Layer {
  name: string;
  order: number;
  segments: Segment[];
}

export interface Snapshot {
  version: number;
  layers: Layer[];
}

export interface LayerResult {
  segment: string;
  strategy: string;
  reason: string;
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
};
