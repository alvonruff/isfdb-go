
# Schema:

The collection feature relies heavily on metadata from the ISFDB pubs table. Basic collection fields:

pub_id              - The pub_id from pubs
col_acq_date           - when the publication was acquired
col_sale_date           - when the publication was sold.

The dates should be strings with default values of 0000-00-00. It is presumed that if the user added a book to their collection,
it is, in fact, acquired. As such, if the acquisition date is zero, it means the user doesn't know or recall the acquistion date.
The sale_date is another matter. If the sale_date is zero it means that the work is still in the user's physical collection. If
the sale date is non-zero, it means that the work was previously in the collection, but not any longer. This allows the user to
say: "Oh yeah, I previously owned that book, but sold it last year"

Optional collector data. This data is for serious collectors who want additional data for value estimations and insurance purposes:

col_cond           - The book's condition;
                      As New / Pristine: An immaculate, unread copy. It has no flaws, wear, or defects, exactly as it left the publisher.
                      Fine (F): A highly collectible copy that is nearly "As New," but may have been carefully opened and read. It has 
                       sharp corners, no fading, and entirely pristine pages.
                      Near Fine (NF): Close to Fine but with minor imperfections, such as light shelf wear, a small bump, or slight rubbing at the edges.
                      Very Good (VG): Shows small signs of handling and wear, but remains clean and sound. Any defects (e.g., owner names or minor 
                       edge rubbing) are noted. This is often the minimum acceptable condition for collectors of modern rarities.
                      Good (G): The average used book. It is completely intact but shows significant wear, such as scuffed boards, minor tears in 
                       the dust jacket, or yellowing pages.
                      Fair: Shows heavy wear and tear but remains complete. The binding may be loose, but all essential text and plates 
                       are present. Typically not suitable for serious collecting.
                      Poor: Heavily damaged, missing pages, or stained. Only of interest if it's an exceptionally rare reading copy.
col_signature       - Contains a signature, presumably by the author/editor.
                      y/n
col_marginalia      - Contains handwritten notes or annotations
                      y/n
col_source          - Plaintext indicating where the book was acquired
col_prch_price      - The purchase price. Currency value
col_ins_value       - Insurance value. Currency value
col_location        - Location within private library. Plaintext
col_note            - Covers any other information of import. Plaintext

# Updates to Existing Scripts

There is a new section in the navbar called "Collection". The links presented there are context sensitive on which
page is currently being viewed. If the user is not viewing a relevant page, then the Collection section should not
be displayed (That is no label in the navbar unless a link is being displayed in that section).

# Scripts:

1. collection_new.cgi

        When viewing a specific publication, a link in the navbar appears in a new section called "Collection". 
        The link label is "Add Pub to Collection" 

        The script presents a form for the user to fill out. Since every field in the schema is optional (except
        for the pub_id, which is passed as an argument in the link), the form should be filled with defaults.
        The form will need a submit button, which will generate a POST request to collection_submitnew.cgi

        Fields that have specific requirements (condition, signature, marginalia) should have prefilled menu selectors.

        An entry consisting of ony default values means that the user owns the pub, and nothing more.

2. collection_submitnew.cgi

        This script extracts the data from the submitted input form, performs some light value checking, and then
        inserts it into the database.

        On failure, an apprppiate error message is displayed.
        On success, it shows a listing of the collection, starting with the largest key_id (most recently entered). There is probably
        a maximum of 200 entries on the page. This code will likely be shared with the next script.

3. collection_list.cgi

        This script displays the contents of the collection, starting with the largest key_id (most recently entered).
        This should be a table, with one work per row. The columns should reflect the database schema, although plaintext fields
        should only display the first 10 characters. The key_id of the collection item should be a clickable link, which
        takes the user to collection_view.cgi

4. collection_view.cgi

        Shows the detail of a specific item in the collection. Most useful for items that have extensive plaintext notes. This
        script takes an argument which is the key_id of the collection item. When this script runs an optional link appears in the
        navbar in the "Collection" section as "Edit Collection Item". This links to collection_edit.cgi

5. collection_edit.cgi

        This script takes a collection key_id and allows the user to make edits. For instance, they may have sold the book, or found
        the book's original receipt with the acquistion date, or maybe the book was damaged in some event. Afer entering data,
        the user presses 'Submit' which takes them to collection_submit.cgi. I presume this is a different script than collection_submitnew.cgi
        as this needs to perform an SQL update, while the previous script needs to perform an SQL insert.

6. collection_search.cgi

        The navbar, in the "Collection" section, should have a link to "Search Collection". This leads to collection_search.cgi
        which is a form with search terms. Some of these actually 

                By Acquisition Date (year of/month of). May be a specific date, or dates before/after a target date.
                By Sale Date (year of/month of). May be a specific date, or dates before/after a target date.

        Others are more complex search terms, as they involve information contained within pub records:

                By Author
                By Title
                By ISBN

        These cannot be simple JOINs, as the pub records are in a different database.

7. collection_slist.cgi

        This script displays the results of a search, with a format similar to collection_list.cgi



