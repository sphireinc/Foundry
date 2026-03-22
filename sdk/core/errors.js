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

export class FoundryAuthError extends FoundryError {
  constructor(message, options = {}) {
    super(message, { ...options, name: 'FoundryAuthError' });
  }
}

export class FoundryValidationError extends FoundryError {
  constructor(message, options = {}) {
    super(message, { ...options, name: 'FoundryValidationError' });
  }
}

export class FoundryNotFoundError extends FoundryError {
  constructor(message, options = {}) {
    super(message, { ...options, name: 'FoundryNotFoundError' });
  }
}

export class FoundryUnsupportedError extends FoundryError {
  constructor(message, options = {}) {
    super(message, { ...options, name: 'FoundryUnsupportedError' });
  }
}

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
