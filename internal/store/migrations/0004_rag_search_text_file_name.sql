-- SPEC-CORE-72.R7: search_text also holds the words of the file name, so every
-- page of a document matches the words of its title. New chunks get it from Go
-- (rag.searchText); this updates the rows indexed before, with the same rule:
-- drop the extension, split on "_", "-", "." and spaces, lowercase and remove
-- accents. tsv is GENERATED from content and search_text, so Postgres
-- recomputes it and nothing needs to be re-ingested.
UPDATE rag_chunks AS c
SET search_text = c.search_text || ' ' || w.words
FROM (
    SELECT chunk_id,
           array_to_string(ARRAY(
               SELECT lower(translate(part,
                   'ÁÀÄÂÃÉÈËÊÍÌÏÎÓÒÖÔÕÚÙÜÛÑÇáàäâãéèëêíìïîóòöôõúùüûñç',
                   'AAAAAEEEEIIIIOOOOOUUUUNCaaaaaeeeeiiiiooooouuuunc'))
               FROM regexp_split_to_table(regexp_replace(file_name, '\.[^./]*$', ''), '[_.[:space:]-]+') AS part
               WHERE part <> ''), ' ') AS words
    FROM rag_chunks
) AS w
WHERE c.chunk_id = w.chunk_id AND w.words <> '';
