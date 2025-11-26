import axios from 'axios';

const BASE_URL = (import.meta.env.VITE_API_BASE_URL || '/api/v1') + '/files';

export default {
  /**
   * Fetches the full recursive directory tree for a given vault.
   * Returns all files and folders with their complete hierarchy.
   * @param {string} vaultId - The ID of the vault.
   * @returns {Promise<Array>} - A promise that resolves to an array of tree nodes.
   */
  getTree(vaultId) {
    return axios.get(`${BASE_URL}/tree/${vaultId}`)
      .then(response => {
        // API returns: { data: { vault_id, nodes, count } }
        // nodes is the full recursive tree structure
        if (response.data && response.data.data && response.data.data.nodes) {
          return response.data.data.nodes;
        }
        return [];
      });
  },

  /**
   * Fetches metadata for a file or directory.
   * @param {string} vaultId - The ID of the vault.
   * @param {string} path - The file or directory path.
   * @returns {Promise<object>} - A promise that resolves to the metadata.
   */
  getMetadata(vaultId, path) {
    return axios.get(`${BASE_URL}/meta/${vaultId}`, { params: { path } })
      .then(response => {
        // API returns: { data: { path, name, type, ... } }
        if (response.data && response.data.data) {
          return response.data.data;
        }
        return null;
      });
  },
  /**
   * Fetches the content of a file by its node ID.
   * @param {string} vaultId - The ID of the vault.
   * @param {string} fileId - The file node ID from the database.
   * @returns {Promise<Object>} - A promise that resolves to an object with content, path, id, and name.
   */
  getFileContent(vaultId, fileId) {
    // Use the by-id endpoint: /api/v1/files/by-id/{vault}/{id}
    // SECURITY: This endpoint only accepts ID, never paths. Path is returned read-only for UI navigation.
    const url = `${BASE_URL}/by-id/${vaultId}/${fileId}`;
    return axios.get(url, {
      responseType: 'json'  // Expect JSON response with content and metadata
    })
      .then(response => {
        // API returns { data: { content, path, id, name } }
        // path is READ-ONLY and used for UI navigation only
        if (response.data && response.data.data) {
          return response.data.data;
        }
        return null;
      });
  },
};
