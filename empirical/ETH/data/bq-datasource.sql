SELECT
    blocks.number, blocks.hash, blocks.timestamp, blocks.difficulty
FROM
    `bigquery-public-data.crypto_ethereum.blocks` as blocks
WHERE
        blocks.number >= 13000000 AND blocks.number <= 14000000
ORDER BY blocks.number ASC
    LIMIT
  10000000