module.exports = {
  default: {
    require: ['e2e/steps/*.cjs'],
    format: ['progress-bar'],
    parallel: 1,
    paths: ['e2e/features/**/*.feature'],
  },
};
