// FoundryError is the base error type returned by SDK requests.
export class FoundryError extends Error {
  constructor(message, options = {}) {
    super(String(message || 'Foundry request failed'));
    this.name = options.name || 'FoundryError';
    this.status = options.status ?? 0;
    this.code = options.code || '';
    this.payload = options.payload ?? null;
    this.details = options.details ?? null;
    this.cause = options.cause;
  }
}

// FoundryAuthError represents authentication or authorization failures.
export class FoundryAuthError extends FoundryError {
  constructor(message, options = {}) {
    super(message, { ...options, name: 'FoundryAuthError' });
  }
}

// FoundryValidationError represents invalid input or conflict-style failures.
export class FoundryValidationError extends FoundryError {
  constructor(message, options = {}) {
    super(message, { ...options, name: 'FoundryValidationError' });
  }
}

// FoundryNotFoundError represents missing resources.
export class FoundryNotFoundError extends FoundryError {
  constructor(message, options = {}) {
    super(message, { ...options, name: 'FoundryNotFoundError' });
  }
}

// FoundryUnsupportedError is used when a stable SDK method exists but the
// backing platform capability is not implemented yet.
export class FoundryUnsupportedError extends FoundryError {
  constructor(message, options = {}) {
    super(message, { ...options, name: 'FoundryUnsupportedError' });
  }
}

// normalizeFoundryError converts transport failures and HTTP responses into the
// small stable error hierarchy used across both SDKs.
export const normalizeFoundryError = ({ response, payload, fallbackMessage, cause } = {}) => {
  const message = payload?.error || payload?.message || fallbackMessage || 'Foundry request failed';
  const options = {
    status: response?.status ?? 0,
    payload,
    details: payload?.details || payload?.errors || null,
    cause,
  };

  if (response?.status === 401 || response?.status === 403) {
    return new FoundryAuthError(message, options);
  }
  if (response?.status === 404) {
    return new FoundryNotFoundError(message, options);
  }
  if (response?.status === 400 || response?.status === 409 || response?.status === 422) {
    return new FoundryValidationError(message, options);
  }
  return new FoundryError(message, options);
};
