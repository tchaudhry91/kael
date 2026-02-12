#!/usr/bin/env node
/**
 * Resolve DNS records for a domain. Returns A, AAAA, MX, NS, TXT, and CNAME records.
 * Input: JSON on stdin with { domain, record_types? }
 * Output: JSON with structured DNS results
 */
const dns = require("dns").promises;

async function resolveType(domain, type) {
  try {
    const records = await dns.resolve(domain, type);
    return records;
  } catch {
    return null;
  }
}

async function main() {
  let input = "";
  for await (const chunk of process.stdin) {
    input += chunk;
  }

  const params = JSON.parse(input);
  const domain = params.domain;
  if (!domain) {
    console.log(JSON.stringify({ error: "domain is required" }));
    process.exit(1);
  }

  const defaultTypes = ["A", "AAAA", "MX", "NS", "TXT", "CNAME"];
  const types = params.record_types || defaultTypes;

  const result = { domain, records: {} };

  for (const type of types) {
    const records = await resolveType(domain, type);
    if (records) {
      result.records[type] = records;
    }
  }

  // Reverse lookup for the first A record
  if (result.records.A && result.records.A.length > 0) {
    try {
      const hostnames = await dns.reverse(result.records.A[0]);
      result.reverse = hostnames;
    } catch {
      // no reverse DNS
    }
  }

  console.log(JSON.stringify(result));
}

main();
